package logic

import (
	"bluebell/dao/kafka"
	"bluebell/dao/localcache"
	"bluebell/dao/mysql"
	"bluebell/dao/rebuild"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/models"
	"bluebell/objects"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
)

var CommentIndexGrp singleflight.Group
var CommentContentGrp singleflight.Group
var CommentMetaDataGrp singleflight.Group

func CreateComment(param *models.ParamCommentCreate, userID int64) (*models.CommentDTO, error) {
	commentID := utils.GenSnowflakeID()
	// 异步投递消息到 kafka
	go func() {
		if err := kafka.CreateComment(*param, userID, commentID); err != nil {
			logger.Errorf("logic:CreateComment: send message to kafka failed, reason: %v", err.Error())
		}
	}()

	commentDTO := models.CommentDTO{
		CommentID: commentID,
		ObjID:     param.ObjID,
		Type:      param.ObjType,
		Root:      param.Root,
		Parent:    param.Parent,
		UserID:    userID,
		// Floor:     floor[0],
		Content: struct {
			Message string "json:\"message\""
		}{
			Message: param.Message,
		},
		CreatedAt: models.Time(time.Now()),
		UpdatedAt: models.Time(time.Now()),
	}

	return &commentDTO, nil
}

// 默认按照楼层排序
func GetCommentList(param *models.ParamCommentList) (*models.CommentListDTO, error) {
	commentIDs, err := getCommentIDs(param.ObjType, param.ObjID, param.PageNum, param.PageSize)
	// logger.Debugf("getCommentIDs: commentIDs: %v", commentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentIDs")
	}
	total, err := redis.GetCommentIndexMemberCount(param.ObjType, param.ObjID) // 根评论总数
	if err != nil {
		return nil, errors.Wrap(err, "logic.GetCommentList.GetCommentIndexMemberCount")
	}
	if total == 0 || len(commentIDs) == 0 {
		return &models.CommentListDTO{Total: total}, nil
	}

	rootCommentIDs := commentIDs // 分页，减少查询成本

	rootCommentDTO, err := GetCommentDetailByCommentIDs(true, true, rootCommentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentDetailByCommentIDs")
	}

	mapping := make(map[int64]int) // 建立映射
	for i := 0; i < len(rootCommentDTO); i++ {
		mapping[rootCommentDTO[i].CommentID] = i
	}

	replies, err := GetCommentDetailByCommentIDs(false, false, rootCommentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentDetailByCommentIDs")
	}

	// 组装数据
	for i := 0; i < len(replies); i++ {
		index, ok := mapping[replies[i].Root]
		if !ok {
			return nil, errors.Wrap(bluebell.ErrInternal, "logic:GetCommentList: get mapping[replies[i].Root] failed")
		}
		rootCommentDTO[index].Replies = append(rootCommentDTO[index].Replies, replies[i])
	}

	if param.OrderBy == "like" {
		// 按 like 降序
		sort.Slice(rootCommentDTO, func(i, j int) bool {
			return rootCommentDTO[i].Like > rootCommentDTO[j].Like
		})
		for i := 0; i < len(rootCommentDTO); i++ {
			sort.Slice(rootCommentDTO[i].Replies, func(a, b int) bool {
				return rootCommentDTO[i].Replies[a].Like > rootCommentDTO[i].Replies[b].Like
			})
		}
	}

	// 为什么不在 mysql 分页？
	// 因为不好建立缓存
	list := &models.CommentListDTO{
		Total:    total,
		Comments: rootCommentDTO,
	}
	return list, nil
}

func RemoveComment(params *models.ParamCommentRemove, userID int64) error {
	// 鉴权处理
	_userID, err := mysql.SelectUserIDByCommentID(nil, params.CommentID)
	if err != nil {
		return errors.Wrap(err, "logic:RemoveComment: SelectUserIDByCommentID")
	}
	if userID != _userID { // 非法操作
		return bluebell.ErrForbidden
	}

	// 判断是不是根评论
	isRoot, err := mysql.CheckIsRootComment(nil, params.CommentID)
	if err != nil {
		return errors.Wrap(err, "logic:RemoveComment: CheckIsRootComment")
	}
	field := "root"
	if !isRoot {
		field = "parent"
	}

	// 获取待删除评论的 ID
	commentIDs, err := mysql.SelectSubCommentIDsByField(nil, params.CommentID, field)
	if err != nil {
		return errors.Wrap(err, "logic:RemoveComment: SelectSubCommentIDsByField")
	}
	commentIDs = append(commentIDs, params.CommentID)
	
	go func() {
		if err := kafka.RemoveComment(*params, userID, commentIDs, isRoot); err != nil {
			logger.Errorf("logic:RemoveComment: send message to kafka failed, reason: %v", err.Error())
		}
	}()

	return nil
}

func RemoveCommentsByObjID(objID int64, objType int8) error {
	go func() {
		if err := kafka.RemoveCommentsByObjID(objID, objType); err != nil {
			logger.Errorf("logic:RemoveCommentsByObjID: send message to kafka failed, reason: %v", err.Error())
		}
	}()

	return nil
}

var (
	commentCache = make(map[string]*sync.Mutex)
	cacheMutex    sync.Mutex
)

// 针对每个 uid_cid_oid_otype 有一个锁
func getCommentMutex(uid_cid_oid_otype string) *sync.Mutex {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	mutex, exists := commentCache[uid_cid_oid_otype]
	if !exists {
		mutex = &sync.Mutex{}
		commentCache[uid_cid_oid_otype] = mutex
	}

	mutex.Lock() // 上了锁以后再给调用者
	return mutex
}

// 在不需要锁的时候释放，避免内存泄漏
func deleteCommentMutex(uid_cid_oid_otype string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	mutex, exist := commentCache[uid_cid_oid_otype]
	if !exist {
		return
	}
	mutex.Lock() // 确保此时已经没有 goroutine 持有该锁，否则针对相同的评论，会有不同的 goroutine 拿到不同的锁，起不到效果
	defer mutex.Unlock()
	delete(commentCache, uid_cid_oid_otype)
}

func LikeOrHateForComment(userID, commentID, objID int64, objType int8, like bool) error {
	key := fmt.Sprintf("%d_%d_%d_%d", userID, commentID, objID, objType)
	mutex := getCommentMutex(key) // 保证拿到的 mutex 已经是上锁状态
	// mutex.Lock()				  // 不要在这里上锁，如果在这里上锁，发生调度，调度到 deleteCommentMutex，将该锁删除，仍可能让后续 goroutine 获取到不同的锁
	defer deleteCommentMutex(key)
	defer mutex.Unlock()		  // 先释放锁，再 deleteCommentMutex，不然死锁

	// 判断缓存是否 miss，如果 miss，重建
	if err := rebuild.RebuildCommentIndex(objType, objID); err != nil {
		return errors.Wrap(err, "logic.LikeOrHateForComment.RebuildCommentIndex")
	}

	// 判断评论是否存在
	exist, err := redis.CheckCommentIfExist(objType, objID, commentID)
	if err != nil {
		return errors.Wrap(err, "logic.LikeOrHateForComment.CheckCommentIfExist")
	}
	if !exist {
		return bluebell.ErrNoSuchComment
	}
	
	// 判断该用户是否点赞（踩）过
	pre, err := redis.CheckCommentLikeOrHateIfExistUser(commentID, userID, objID, objType, like)
	if err != nil {
		return errors.Wrap(err, "logic:LikeOrHateForComment: CheckCommentLikeOrHateIfExistUser")
	}

	if !pre { // 可能没有点赞过
		// check if cache miss
		key := redis.KeyCommentLikeSetPF
		if !like {
			key = redis.KeyCommentHateSetPF
		}
		// key = key + strconv.FormatInt(commentID, 10)
		key = fmt.Sprintf("%s%d_%d_%d", key, commentID, objID, objType)
		exist, err := redis.Exists(key)
		if err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: Exists")
		}
		if !exist {
			// cache miss,
			// 由于加了缓存，bluebell:comment:rem:cid 可能还没来得及持久化到 db（删除 cid），如果直接重建，会获取到脏数据
			// 先检查一下，确定是否重建
			exist2, err := redis.CheckCommentRemCidIfExistCid(commentID)

			if err == nil && exist2 { // 说明用户尝试过取消点赞，但还没来得及持久化到 db 的 ciduid 表
				pre = false
			} else { // 不存在，cache rebuild
				pre, err = rebuild.RebuildCommentLikeOrHateSet(commentID, userID, objID, objType, like)
				if err != nil {
					return errors.Wrap(err, "logic:LikeOrHateForComment: RebuildCommentLikeOrHateSet")
				}
			}
		}
	}

	if pre { // 取消点赞（踩）
		if err := redis.RemCommentLikeOrHateUser(commentID, userID, objID, objType, like); err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: RemCommentLikeOrHateUser")
		}
		// 还要删除 db 的 cid_uid
		// 这里添加到缓存，由后台任务负责删除
		if err := redis.AddCommentRemCid(commentID); err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: AddCommentRemCidUid")
		}

		// 还要删除缓存 bluebell:comment:userlikeids:
		if err := redis.RemCommentUserLikeOrHateMapping(userID, commentID, objID, objType, like); err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: RemCommentLikeOrHateUser")
		}
	} else { // 点赞（踩）
		// 先删可能存在的 bluebell:comment:rem:cid_uid（用户之前取消过点赞）
		// 防止后台任务将我们刚刚添加的 cid_uid 从 db 删掉（这样会导致可以重复点赞）
		if err := redis.RemCommentRemCid(commentID); err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: RemCommentRemCidUid")
		}
		if err := redis.AddCommentLikeOrHateUser(commentID, userID, objID, objType, like); err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: AddCommentLikeOrHateUser")
		}

		// 写缓存 bluebell:comment:userlike(hate)ids:
		// 尝试重建，由 rebuild 判断需不需要重建
		_, _, err = rebuild.RebuildCommentUserLikeOrHateMapping(userID, objID, objType, like)
		if err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: RebuildCommentUserLikeOrHateMapping")
		}

		err = redis.AddCommentUserLikeOrHateMappingByCommentIDs(userID, objID, objType, like, []int64{commentID})
		if err != nil {
			return errors.Wrap(err, "logic:LikeOrHateForComment: AddCommentUserLikeOrHateMapping")
		}
	}

	offset := 1
	if pre {
		offset = -1
	}

	if err = redis.IncrCommentLikeOrHateCount(commentID, offset, like); err != nil {
		return errors.Wrap(err, "logic:LikeOrHateForComment: IncrCommentIndexCountField")
	}

	// 更新缓存(if exists)
	cacheKey := fmt.Sprintf("%v_%v_metadata", objects.ObjComment, commentID)
	comment, err := localcache.GetLocalCache().Get(cacheKey)
	if err == nil {
		tmp := comment.(models.CommentDTO)
		tmp.Like += offset
		if err = localcache.GetLocalCache().Set(cacheKey, tmp); err != nil {
			logger.Warnf("logic:LikeOrHateForComment: Update local cache failed, reason: %v", err.Error())
		}
	}

	return nil
}

func GetCommentUserLikeOrHateList(userID int64, params *models.ParamCommentUserLikeOrHateList) ([]string, error) {
	list, rebuilt, err := rebuild.RebuildCommentUserLikeOrHateMapping(userID, params.ObjID, params.ObjType, params.Like)
	if err != nil {
		logger.Warnf("logic:GetCommentUserLikeOrHateList: RebuildCommentUserLikeOrHateMapping failed, reason: %s, reading db...", err.Error()) // 重建失败，读 db
		list, err = mysql.SelectCommentUserLikeOrHateList(nil, userID, params.ObjID, params.ObjType, params.Like)
		if err != nil { // 读 db 失败，请求失败
			return nil, errors.Wrap(err, "logic:GetCommentUserLikeOrHateList: SelectCommentUserLikeOrHateList")
		}
	} else if !rebuilt { // 没有重建，读 cache
		list, err = redis.GetCommentUserLikeOrHateList(userID, params.ObjID, params.ObjType, params.Like)
		if err != nil { // 读 cache 失败，尝试读 db
			logger.Warnf("logic:GetCommentUserLikeOrHateList: RebuildCommentUserLikeOrHateMapping failed, reason: %s, reading db...", err.Error()) // 重建失败，读 db
			list, err = mysql.SelectCommentUserLikeOrHateList(nil, userID, params.ObjID, params.ObjType, params.Like)
			if err != nil { // 读 db 失败，请求失败
				return nil, errors.Wrap(err, "logic:GetCommentUserLikeOrHateList: SelectCommentUserLikeOrHateList")
			}
		}
	}
	listStr := make([]string, len(list))
	for i := 0; i < len(list); i++ {
		listStr[i] = strconv.FormatInt(list[i], 10)
	}
	return listStr, nil
}

func GetCommentDetailByCommentIDs(isRoot, needIncrView bool, commentIDs []int64) ([]models.CommentDTO, error) {
	var commentDTOList []models.CommentDTO
	missCommentIDs := make([]int64, 0, len(commentIDs))
	if isRoot {
		commentDTOList = make([]models.CommentDTO, len(commentIDs))
		// 调用者要求查询 commentIDs 对应的 metadata
		for idx, commentID := range commentIDs {
			commentIDStr := strconv.FormatInt(commentID, 10)
			// 递增 view
			if needIncrView {
				isNewMember, err := localcache.IncrView(objects.ObjComment, commentID, 1)
				if err != nil {
					logger.Warnf("logic:getCommentDetailByCommentIDs: IncrView failed(incr comment view)")
				} else if isNewMember {
					if err := localcache.SetViewCreateTime(objects.ObjComment, commentID, time.Now().Unix()); err != nil {
						logger.Warnf("logic:GetPostDetailByID: SetViewCreateTime(comment) failed")
						// 应该保证事务一致性原则（回滚 incr 操作）
						// 这里简单处理，不考虑回滚失败
						localcache.IncrView(objects.ObjComment, commentID, -1)
					}
				}
			}

			// 查 local cache，获取 metadata
			cacheKey := fmt.Sprintf("%v_%v_metadata", objects.ObjComment, commentIDStr) // 用于获取 local cache 的 key
			commentDTO, err := localcache.GetLocalCache().Get(cacheKey)
			if err == nil { // cache hit
				commentDTOList[idx] = commentDTO.(models.CommentDTO)
			} else { // cache miss
				commentDTOList[idx].CommentID = -1
				missCommentIDs = append(missCommentIDs, commentID)
			}
		}
	} else {
		commentDTOList = make([]models.CommentDTO, 0, len(commentIDs))
		// 调用者要求查询 commentIDs 的子评论列表
		for _, commentID := range commentIDs {
			cacheKey := fmt.Sprintf("%v_%v_replies", objects.ObjComment, commentID)

			replyList, err := localcache.GetLocalCache().Get(cacheKey)
			if err == nil { // cache hit
				// 在 local cache 中获取子评论的 metadata
				replyCommentIDs := replyList.([]int64)
				replyMetadata, err := GetCommentDetailByCommentIDs(true, false, replyCommentIDs)
				if err != nil {
					logger.Warnf("logic:GetCommentDetailByCommentIDs: get reply metadata failed, reason: %v", err.Error())
					missCommentIDs = append(missCommentIDs, commentID)
				} else {
					commentDTOList = append(commentDTOList, replyMetadata...)
				}
			} else { // cache miss
				missCommentIDs = append(missCommentIDs, commentID)
			}
		}
	}

	if len(missCommentIDs) == 0 { // all hit
		return commentDTOList, nil
	}
	// 现在只需要查询 missCommentIDs 的元数据

	field := "id"
	if !isRoot {
		field = "root"
	}

	missCommentIDStrs := utils.ConvertInt64SliceToStringSlice(missCommentIDs)
	sfkey := strings.Join(missCommentIDStrs, "_")
	timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
	rps := viper.GetInt("service.rps")
	interval := time.Second / time.Duration(rps)
	_missCommentDTOList, err := utils.SfDoWithTimeout(&CommentMetaDataGrp, sfkey, timeout, interval, func() (any, error) {
		return mysql.SelectCommentMetaDataByCommentIDs(nil, field, missCommentIDs)
	})
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: SelectCommentMetaDataByCommentIDs failed")
	}
	missCommentDTOList := _missCommentDTOList.([]models.CommentDTO)

	// 查点赞数
	if !isRoot { // 子评论，需要获取子评论 id
		replyIDs := make([]int64, 0, len(missCommentDTOList))
		for _, reply := range missCommentDTOList {
			replyIDs = append(replyIDs, reply.CommentID)
		}
		missCommentIDs = replyIDs
	}
	likes, err := redis.GetCommentLikeOrHateCountByCommentIDs(missCommentIDs, true)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: GetCommentLikeOrHateCountByCommentIDs failed")
	}

	// 查 content
	contents, err := getCommentContent(missCommentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentContent failed")
	}
	if len(contents) != len(missCommentDTOList) {
		return nil, errors.Wrap(bluebell.ErrInternal, "logic:GetCommentList: contents and missCommentDTOList length is not equal")
	}

	// 组装数据
	for i := 0; i < len(missCommentDTOList); i++ {
		missCommentDTOList[i].Content.Message = contents[i]
		missCommentDTOList[i].Like += likes[i]
	}

	if isRoot {
		j := 0
		for i := 0; i < len(commentDTOList); i++ {
			if commentDTOList[i].CommentID == -1 {
				if j >= len(missCommentDTOList) {
					logger.Warnf("logic:GetCommentDetailByCommentIDs: len(missCommentDTOList) invalid, check if has expired comment_id in local cache(view)")
					break
				}
				commentDTOList[i] = missCommentDTOList[j]
				j++
			}
		}
	} else {
		commentDTOList = append(commentDTOList, missCommentDTOList...)
	}

	return commentDTOList, nil
}

func getCommentIDs(objType int8, objID, pageNum, pageSize int64) ([]int64, error) {
	key := fmt.Sprintf("%v%v_%v", redis.KeyCommentIndexZSetPF, objType, objID)
	exist, err := redis.Exists(key)
	if err != nil {
		return nil, errors.Wrap(err, "logic:getCommentIDs: Exists")
	}

	start := (pageNum - 1) * pageSize
	stop := start + pageSize
	if exist { // cache hit
		return redis.GetCommentIndexMember(objType, objID, start, stop)
	} else { // cache miss, rebuild
		key = fmt.Sprintf("%v_%v", objType, objID)
		timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
		rps := viper.GetInt("service.rps")
		interval := time.Second / time.Duration(rps)

		commentIDs, err := utils.SfDoWithTimeout(&CommentIndexGrp, key, timeout, interval, func() (any, error) {
			// 检查缓存是否 miss，如果 miss，重建
			if err := rebuild.RebuildCommentIndex(objType, objID); err != nil {
				return nil, errors.Wrap(err, "logic.getCommentIDs.RebuildCommentIndex")
			}
			return redis.GetCommentIndexMember(objType, objID, start, stop)
		})
		if err != nil {
			return nil, errors.Wrap(err, "logic:getCommentIDs: RebuildCommentIndex")
		}
		return commentIDs.([]int64), nil
	}
}

func getCommentContent(commentIDs []int64) ([]string, error) {
	commentIDStrs := utils.ConvertInt64SliceToStringSlice(commentIDs)

	sfkey := strings.Join(commentIDStrs, "_")
	timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
	rps := viper.GetInt("service.rps")
	interval := time.Second / time.Duration(rps)

	_, err := utils.SfDoWithTimeout(&CommentContentGrp, sfkey, timeout, interval, func() (any, error) {
		return nil, rebuild.RebuildCommentContent(commentIDs)
	})
	if err != nil {
		return nil, errors.Wrap(err, "logic:getCommentContent: RebuildCommentContent")
	}

	// rebuild 成功（或者不需要 rebuild），读缓存
	return redis.GetCommentContents(commentIDs)
}
