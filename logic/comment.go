package logic

import (
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
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

var CommentIndexGrp singleflight.Group
var CommentContentGrp singleflight.Group
var CommentMetaDataGrp singleflight.Group

func CreateComment(param *models.ParamCommentCreate, userID int64) (*models.CommentDTO, error) {
	commentID := utils.GenSnowflakeID()
	commentContent := &models.CommentContent{
		CommentID: commentID,
		Message:   param.Message,
	}

	// 先写 content（弱一致性需求）
	if err := mysql.CreateCommentContent(commentContent); err != nil {
		return nil, errors.Wrap(err, "logic:CreateComment: CreateCommentContent failed")
	}

	// 事务更新 subject、index 表
	tx0 := mysql.GetDB().Begin()

	// 判断是否需要创建新的 subject
	exist, err := mysql.SelectCommentSubjectCount(tx0, param.ObjID, param.ObjType)
	if err != nil {
		tx0.Rollback()
		return nil, errors.Wrap(err, "logic:CreateComment: SelectCommentSubjectCount failed")
	}
	if exist == 0 {
		if err := mysql.CreateCommentSubject(tx0, commentID, param.ObjID, param.ObjType); err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) { // 其它错误，事务失败
				tx0.Rollback()
				return nil, errors.Wrap(err, "logic:CreateComment: SelectCommentSubjectCount failed")
			}
		}
	}

	// 如果是根评论，递增 root_count、count（update 实际上是串行执行的，因为有锁，因此，如果有其他事务还没提交，这里就会阻塞等待）
	if param.Root == 0 {
		if err := mysql.IncrCommentSubjectCountField(tx0, "root_count", param.ObjID, param.ObjType, 1); err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: IncrCommentSubjectRootCount(root_count) failed")
		}

		if err := mysql.IncrCommentSubjectCountField(tx0, "count", param.ObjID, param.ObjType, 1); err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: IncrCommentSubjectCount(count) failed")
		}
	}

	// 确定 floor 字段
	var floor int
	if param.Root == 0 {
		floor, err = mysql.SelectCommentSubjectCountField(tx0, "count", param.ObjID, param.ObjType)
		if err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: SelectCommentSubjectCount(count) failed")
		}
	} else {
		err = mysql.IncrCommentIndexCountField(tx0, "count", param.Root, 1)
		if err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: IncrCommentIndexCount(count) failed")
		}
		err = mysql.IncrCommentIndexCountField(tx0, "root_count", param.Root, 1)
		if err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: IncrCommentIndexCount(root_count) failed")
		}
		floor, err = mysql.SelectCommentIndexCountField(tx0, "count", param.Root)
		if err != nil {
			tx0.Rollback()
			return nil, errors.Wrap(err, "logic:CreateComment: SelectCommentIndexCount(count) failed")
		}
	}

	// 写 index 表
	index := &models.CommentIndex{
		ID:        commentID,
		ObjID:     param.ObjID,
		ObjType:   param.ObjType,
		Root:      param.Root,
		Parent:    param.Parent,
		UserID:    userID,
		Floor:     floor,
		Count:     0,
		RootCount: 0,
	}

	if err = mysql.CreateCommentIndex(tx0, index); err != nil {
		tx0.Rollback()
		return nil, errors.Wrap(err, "logic:CreateComment: CreateCommentIndex failed")
	}

	tx0.Commit()

	commentDTO := models.CommentDTO{
		CommentID: commentID,
		ObjID:     param.ObjID,
		Type:      param.ObjType,
		Root:      param.Root,
		Parent:    param.Parent,
		UserID:    userID,
		Floor:     floor,
		Content: struct {
			Message string "json:\"message\""
		}{
			Message: param.Message,
		},
		CreatedAt: models.Time(time.Now()),
		UpdatedAt: models.Time(time.Now()),
	}

	// 写缓存
	if index.Root == 0 {
		_, err = rebuild.RebuildCommentIndex(index.ObjType, index.ObjID, 0) // 在写缓存前尝试 rebuild 一下，确保缓存中有完整的 comment_id
		if err != nil {
			// 重建失败，如果继续写缓存，可能会造成缓存中不具有完整的 comment_id，拒绝服务
			return nil, errors.Wrap(err, "logic:CreateComment: RebuildCommentIndex")
		}
		if err = redis.AddCommentIndexMembers(index.ObjType, index.ObjID, []int64{commentID}, []int{floor}); err != nil {
			logger.Warnf("logic:CreateComment: AddCommentIndexMember failed, reason: %v", err.Error())
		}
	} else { // 判断是否需要更新 local cache
		cacheKey := fmt.Sprintf("%v_%v_replies", objects.ObjComment, index.Root)
		replyIDs, err := localcache.GetLocalCache().Get(cacheKey)
		if err == nil { // cache hit，need update
			tmp := replyIDs.([]int64)
			tmp = append(tmp, commentID)
			localcache.GetLocalCache().Set(cacheKey, tmp)
			cacheKey = fmt.Sprintf("%v_%v_replies", objects.ObjComment, commentID)
			localcache.GetLocalCache().Set(cacheKey, commentDTO)
		}
	}
	if err = redis.AddCommentContents([]int64{commentID}, []string{commentContent.Message}); err != nil {
		logger.Warnf("logic:CreateComment: AddCommentContent failed, reason: %v", err.Error())
	}

	return &commentDTO, nil
}

// 默认按照楼层排序
func GetCommentList(param *models.ParamCommentList) (*models.CommentListDTO, error) {
	commentIDs, err := getCommentIDs(param.ObjType, param.ObjID)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentIDs")
	}
	total := len(commentIDs) // 根评论总数
	if total == 0 {
		return &models.CommentListDTO{Total: total}, nil
	}

	// 检查分页参数是否正确
	start := (param.PageNum - 1) * param.PageSize
	if start >= int64(total) {
		return nil, errors.Wrap(bluebell.ErrInvalidParam, "logic:GetCommentList: PageNum is too long!")
	}
	end := start + param.PageSize
	if end >= int64(total) {
		end = int64(total)
	}

	rootCommentIDs := commentIDs[start:end] // 分页，减少查询成本

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
	tx := mysql.GetDB().Begin()

	// 修改 root_count（主要是可能要获取根评论 id，删除了就获取不到了，由于是一个事务，顺序其实无所谓）
	offset := len(commentIDs)
	if isRoot {
		// 修改 subject 的 root_count
		if err := mysql.IncrCommentSubjectCountField(tx, "root_count", params.ObjID, params.ObjType, -1); err != nil {
			tx.Rollback()
			return errors.Wrap(err, "logic:RemoveComment: IncrCommentSubjectCountField(root_count)")
		}
	} else {
		// 获取根评论 id
		root, err := mysql.SelectCommentRootIDByCommentID(tx, params.CommentID)
		if err != nil {
			tx.Rollback()
			return errors.Wrap(err, "logic:RemoveComment: SelectCommentRootIDByCommentID")
		}
		if err := mysql.IncrCommentIndexCountField(tx, "root_count", root, -offset); err != nil { // 修改对应根评论的 root_count
			tx.Rollback()
			return errors.Wrap(err, "logic:RemoveComment: IncrCommentIndexCountField(root_count)")
		}
	}

	// ciduids := make([]string, len(commentIDs))
	// for i := 0; i < len(ciduids); i++ {
	// 	ciduids[i] = fmt.Sprintf("%v:%v", commentIDs[i], userID)
	// }

	// 根据 ID 删除评论
	// 事务删除
	if err := mysql.DeleteCommentIndexByCommentIDs(tx, commentIDs); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "logic:RemoveComment: DeleteCommentIndexByCommentIDs")
	}
	if err := mysql.DeleteCommentContentByCommentIDs(tx, commentIDs); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "logic:RemoveComment: DeleteCommentContentByCommentIDs")
	}
	if err := mysql.DeleteCommentUserLikeMappingByCommentIDs(tx, commentIDs); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "logic:RemoveComment: DeleteCommentUserLikeMappingByCommentIDs")
	}
	if err := mysql.DeleteCommentUserHateMappingByCommentIDs(tx, commentIDs); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "logic:RemoveComment: DeleteCommentUserHateMappingByCommentIDs")
	}

	tx.Commit()

	// 删缓存
	if isRoot {
		err = redis.RemCommentIndexMembersByCommentID(params.ObjType, params.ObjID, params.CommentID) // 先把索引删了
		if err != nil {
			logger.Warnf("logic:RemoveComment: RemCommentIndexMembersByCommentIDs failed, reason: %v", err.Error())
		}
	}
	redis.DelCommentContentsByCommentIDs(commentIDs)
	redis.DelCommentLikeOrHateCountByCommentIDs(commentIDs, true)
	redis.DelCommentLikeOrHateCountByCommentIDs(commentIDs, false)
	redis.DelCommentLikeOrHateUserByCommentIDs(commentIDs, params.ObjID, params.ObjType, true)
	redis.DelCommentLikeOrHateUserByCommentIDs(commentIDs, params.ObjID, params.ObjType, false)

	// 删本地缓存
	cacheKey := fmt.Sprintf("%v_%v_metadata", objects.ObjComment, params.CommentID)
	localcache.GetLocalCache().Remove(cacheKey)
	cacheKey = fmt.Sprintf("%v_%v_replies", objects.ObjComment, params.CommentID)
	localcache.GetLocalCache().Remove(cacheKey)
	localcache.RemoveObjectView(objects.ObjComment, params.CommentID)

	return nil
}

func LikeOrHateForComment(userID, commentID, objID int64, objType int8, like bool) error {
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

func getCommentIDs(objType int8, objID int64) ([]int64, error) {
	key := fmt.Sprintf("%v%v_%v", redis.KeyCommentIndexZSetPF, objType, objID)
	exist, err := redis.Exists(key)
	if err != nil {
		return nil, errors.Wrap(err, "logic:getCommentIDs: Exists")
	}

	if exist { // cache hit
		return redis.GetCommentIndexMember(objType, objID)
	} else { // cache miss, rebuild
		key = fmt.Sprintf("%v_%v", objType, objID)
		timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
		rps := viper.GetInt("service.rps")
		interval := time.Second / time.Duration(rps)

		commentIDs, err := utils.SfDoWithTimeout(&CommentIndexGrp, key, timeout, interval, func() (any, error) {
			return rebuild.RebuildCommentIndex(objType, objID, 0)
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
