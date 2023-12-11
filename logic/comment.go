package logic

import (
	"bluebell/dao/mysql"
	"bluebell/dao/rebuild"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/models"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

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

	// 写缓存
	if index.Root == 0 {
		_, _, err = rebuild.RebuildCommentIndex(index.ObjType, index.ObjID, 0) // 在写缓存前尝试 rebuild 一下，确保缓存中有完整的 comment_id
		if err != nil {
			// 重建失败，如果继续写缓存，可能会造成缓存中不具有完整的 comment_id，拒绝服务
			return nil, errors.Wrap(err, "logic:CreateComment: RebuildCommentIndex")
		}
		if err = redis.AddCommentIndexMembers(index.ObjType, index.ObjID, []int64{commentID}, []int{floor}); err != nil {
			logger.Warnf("logic:CreateComment: AddCommentIndexMember failed, reason: %v", err.Error())
		}
	}
	if err = redis.AddCommentContents([]int64{commentID}, []string{commentContent.Message}); err != nil {
		logger.Warnf("logic:CreateComment: AddCommentContent failed, reason: %v", err.Error())
	}

	return &models.CommentDTO{
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
	}, nil
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

	_commentIDs := commentIDs[start:end] // 分页，减少查询成本

	rootCommentDTO, err := getCommentDetailByCommentIDs(true, _commentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentDetailByCommentIDs")
	}

	mapping := make(map[int64]int) // 建立映射
	for i := 0; i < len(rootCommentDTO); i++ {
		mapping[rootCommentDTO[i].CommentID] = i
	}

	replies, err := getCommentDetailByCommentIDs(false, _commentIDs)
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

		return errors.Wrap(redis.IncrCommentLikeOrHateCount(commentID, -1, like), "logic:LikeOrHateForComment: IncrCommentIndexCountField")

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

		return errors.Wrap(redis.IncrCommentLikeOrHateCount(commentID, 1, like), "logic:LikeOrHateForComment: IncrCommentIndexCountField")
	}

	// 无效参数
	// return errors.Wrap(bluebell.ErrInvalidParam, "logic:LikeOrHateForComment: invalid opinion")
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

func getCommentIDs(objType int8, objID int64) ([]int64, error) {
	readDB := func(err error) ([]int64, error) {
		logger.Warnf("logic:GetCommentList: RebuildCommentIndex failed, reason: %v", err.Error())
		return mysql.SelectRootCommentIDs(nil, objType, objID)
	}

	// 先 rebuild 一下，防止 cache miss
	commentIDs, rebuilt, err := rebuild.RebuildCommentIndex(objType, objID, 0)
	if err != nil { // 重建失败，尝试从 mysql 获取数据
		return readDB(err)
	} else if !rebuilt { // 没有重建，需要读缓存
		commentIDs, err = redis.GetCommentIndexMember(objType, objID)
		if err != nil { // 读缓存失败，尝试从 mysql 获取数据
			return readDB(err)
		}
		return commentIDs, nil
	} else { // 重建，并且成功，直接使用重建时从 db 获取的数据，避免再多查一次缓存
		return commentIDs, nil
	}
}

func getCommentDetailByCommentIDs(isRoot bool, commentIDs []int64) ([]models.CommentDTO, error) {
	field := "id"
	if !isRoot {
		field = "root"
	}
	commentDTOList, err := mysql.SelectCommentMetaDataByCommentIDs(nil, field, commentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: SelectCommentMetaDataByCommentIDs failed")
	}

	// 查点赞数
	if !isRoot {  // 子评论，需要获取子评论 id
		replyIDs := make([]int64, 0, len(commentDTOList))
		for _, reply := range commentDTOList {
			replyIDs = append(replyIDs, reply.CommentID)
		}
		logger.Debugf("commentID: %v\nreplyID: %v", commentIDs, replyIDs)
		commentIDs = replyIDs
	}
	likes, err := redis.GetCommentLikeOrHateCountByCommentIDs(commentIDs, true)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: GetCommentLikeOrHateCountByCommentIDs failed")
	}

	// 查 content
	contents, err := getCommentContent(commentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "logic:GetCommentList: getCommentContent failed")
	}
	if len(contents) != len(commentDTOList) {
		return nil, errors.Wrap(bluebell.ErrInternal, "logic:GetCommentList: contents and commentDTOList length is not equal")
	}

	// 组装数据
	for i := 0; i < len(commentDTOList); i++ {
		commentDTOList[i].Content.Message = contents[i]
		commentDTOList[i].Like += likes[i]
	}

	return commentDTOList, nil
}

func getCommentContent(commentIDs []int64) ([]string, error) {
	readDB := func(err error) ([]string, error) {
		logger.Warnf("logic:GetCommentList: RebuildCommentContent failed, reason: %v", err.Error())
		tmp, err := mysql.SelectCommentContentByCommentIDs(nil, commentIDs)
		if err != nil { // 读 db 失败，拒绝服务
			return nil, errors.Wrap(err, "logic:GetCommentList: SelectCommentContentByCommentIDs failed")
		}
		contents := make([]string, len(tmp))
		for i := 0; i < len(tmp); i++ {
			contents[i] = tmp[i].Message
		}
		return contents, nil
	}

	// cache rebuild 一下
	err := rebuild.RebuildCommentContent(commentIDs)
	if err != nil { // rebuild 失败，读 db
		return readDB(err)
	}

	// rebuild 成功（或者不需要 rebuild），读缓存
	return redis.GetCommentContents(commentIDs)
}
