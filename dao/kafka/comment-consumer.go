package kafka

import (
	"bluebell/dao/localcache"
	"bluebell/dao/mysql"
	"bluebell/dao/rebuild"
	"bluebell/dao/redis"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/models"
	"bluebell/objects"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

func GetCommentCreateUniqueKey(commentID int64) string {
	return fmt.Sprintf("create_%v", commentID)
}

func GetCommentRemoveUniqueKey(commentID int64) string {
	return fmt.Sprintf("remove_%v", commentID)
}

func createComment(tx *gorm.DB, msg kafka.Message, params CommentCreate) (res Result) {
	// logger.Debugf("createComment: key: %v, partition: %v, offset: %v, comment_id: %v, content: %v\n", string(msg.Key), msg.Partition, msg.Offset, params.CommentID, params.Message)
	res.UniqueKey = GetCommentCreateUniqueKey(params.CommentID)

	// 与之前的 logic 一致
	commentContent := &models.CommentContent{
		CommentID: params.CommentID,
		Message:   params.Message,
	}

	// 先写 content（弱一致性需求）
	if err := mysql.CreateCommentContent(tx, commentContent); err != nil {
		res.Err = errors.Wrap(err, "kafka:CreateComment: CreateCommentContent")
		return
	}

	// 判断是否需要创建新的 subject
	exist, err := mysql.SelectCommentSubjectCount(tx, params.ObjID, params.ObjType)
	if err != nil {
		res.Err = errors.Wrap(err, "kafka:CreateComment: SelectCommentSubjectCount")
		return
	}
	if exist == 0 {
		if err := mysql.CreateCommentSubject(tx, params.CommentID, params.ObjID, params.ObjType); err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) { // 其它错误，事务失败
				res.Err = errors.Wrap(err, "kafka:CreateComment: SelectCommentSubjectCount")
				return
			}
		}
	}

	// 如果是根评论，递增 root_count、count（update 实际上是串行执行的，因为有锁，因此，如果有其他事务还没提交，这里就会阻塞等待）
	if params.Root == 0 {
		if err := mysql.IncrCommentSubjectCountField(tx, "root_count", params.ObjID, params.ObjType, 1); err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: IncrCommentSubjectRootCount(root_count)")
			return
		}

		if err := mysql.IncrCommentSubjectCountField(tx, "count", params.ObjID, params.ObjType, 1); err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: IncrCommentSubjectCount(count)")
			return
		}
	}

	// 确定 floor 字段
	var floor int
	if params.Root == 0 {
		floor, err = mysql.SelectCommentSubjectCountField(tx, "count", params.ObjID, params.ObjType)
		if err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: SelectCommentSubjectCount(count)")
			return
		}
	} else {
		err = mysql.IncrCommentIndexCountField(tx, "count", params.Root, 1)
		if err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: IncrCommentIndexCount(count)")
			return
		}
		err = mysql.IncrCommentIndexCountField(tx, "root_count", params.Root, 1)
		if err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: IncrCommentIndexCount(root_count)")
			return
		}
		floor, err = mysql.SelectCommentIndexCountField(tx, "count", params.Root)
		if err != nil {
			res.Err = errors.Wrap(err, "kafka:CreateComment: SelectCommentIndexCount(count)")
			return
		}
	}

	// 写 index 表
	index := &models.CommentIndex{
		ID:        params.CommentID,
		ObjID:     params.ObjID,
		ObjType:   params.ObjType,
		Root:      params.Root,
		Parent:    params.Parent,
		UserID:    params.UserID,
		Floor:     floor,
		Count:     0,
		RootCount: 0,
	}

	if err = mysql.CreateCommentIndex(tx, index); err != nil {
		res.Err = errors.Wrap(err, "kafka:CreateComment: CreateCommentIndex")
	}

	// 写缓存
	if params.Root == 0 {
		_, err = rebuild.RebuildCommentIndex(params.ObjType, params.ObjID, 0) // 在写缓存前尝试 rebuild 一下，确保缓存中有完整的 comment_id
		if err != nil {
			// 重建失败，如果继续写缓存，可能会造成缓存中不具有完整的 comment_id，拒绝服务
			res.Err = errors.Wrap(err, "kafka:CreateComment: RebuildCommentIndex")
			return
		}
		if err = redis.AddCommentIndexMembers(params.ObjType, params.ObjID, []int64{params.CommentID}, []int{floor}); err != nil {
			logger.Warnf("kafka:CreateComment: AddCommentIndexMember, reason: %v", err.Error())
		}
	} else { // 判断是否需要更新 local cache
		cacheKey := fmt.Sprintf("%v_%v_replies", objects.ObjComment, params.Root)
		replyIDs, err := localcache.GetLocalCache().Get(cacheKey)
		if err == nil { // cache hit，need update
			tmp := replyIDs.([]int64)
			tmp = append(tmp, params.CommentID)
			localcache.GetLocalCache().Set(cacheKey, tmp)
			cacheKey = fmt.Sprintf("%v_%v_replies", objects.ObjComment, params.CommentID)
			localcache.GetLocalCache().Set(cacheKey, models.CommentDTO{
				CommentID: params.CommentID,
				ObjID:     params.ObjID,
				Type:      params.ObjType,
				Root:      params.Root,
				Parent:    params.Parent,
				UserID:    params.UserID,
				Floor:     floor,
				Content: struct {
					Message string "json:\"message\""
				}{
					Message: params.Message,
				},
				CreatedAt: models.Time(time.Now()),
				UpdatedAt: models.Time(time.Now()),
			})
		}
	}
	if err = redis.AddCommentContents([]int64{params.CommentID}, []string{params.Message}); err != nil {
		logger.Warnf("kafka:CreateComment: AddCommentContent, reason: %v", err.Error())
	}
	return
}

func removeComment(tx *gorm.DB, params CommentRemove) (res Result) {
	// logger.Debugf("removeComment: comment_id: %v\n", params.CommentID)
	res.UniqueKey = GetCommentRemoveUniqueKey(params.CommentID)

	// 修改 root_count（主要是可能要获取根评论 id，删除了就获取不到了，由于是一个事务，顺序其实无所谓）
	offset := len(params.CommentIDs)
	if params.IsRoot {
		// 修改 subject 的 root_count
		if err := mysql.IncrCommentSubjectCountField(tx, "root_count", params.ObjID, params.ObjType, -1); err != nil {
			res.Err = errors.Wrap(err, "kafka:RemoveComment: IncrCommentSubjectCountField(root_count)")
			return
		}
	} else {
		// 获取根评论 id
		root, err := mysql.SelectCommentRootIDByCommentID(tx, params.CommentID)
		if err != nil {
			res.Err = errors.Wrap(err, "kafka:RemoveComment: SelectCommentRootIDByCommentID")
			return
		}
		if err := mysql.IncrCommentIndexCountField(tx, "root_count", root, -offset); err != nil { // 修改对应根评论的 root_count
			res.Err = errors.Wrap(err, "kafka:RemoveComment: IncrCommentIndexCountField(root_count)")
			return
		}
	}

	// 根据 ID 删除评论
	// 事务删除
	commentIDs, err := utils.ConvertStringSliceToInt64Slice(params.CommentIDs)
	if err != nil {
		res.Err = errors.Wrap(err, "kafka:RemoveComment: ConvertStringSliceToInt64Slice")
		return
	}

	if err := mysql.DeleteCommentIndexByCommentIDs(tx, commentIDs); err != nil {
		res.Err = errors.Wrap(err, "kafka:RemoveComment: DeleteCommentIndexByCommentIDs")
		return
	}
	if err := mysql.DeleteCommentContentByCommentIDs(tx, commentIDs); err != nil {
		res.Err = errors.Wrap(err, "kafka:RemoveComment: DeleteCommentContentByCommentIDs")
		return
	}
	if err := mysql.DeleteCommentUserLikeMappingByCommentIDs(tx, commentIDs); err != nil {
		res.Err = errors.Wrap(err, "kafka:RemoveComment: DeleteCommentUserLikeMappingByCommentIDs")
		return
	}
	if err := mysql.DeleteCommentUserHateMappingByCommentIDs(tx, commentIDs); err != nil {
		res.Err = errors.Wrap(err, "kafka:RemoveComment: DeleteCommentUserHateMappingByCommentIDs")
	}

	// 删缓存
	if params.IsRoot {
		err := redis.RemCommentIndexMembersByCommentID(params.ObjType, params.ObjID, params.CommentID) // 先把索引删了
		if err != nil {
			logger.Warnf("kafka:RemoveComment: RemCommentIndexMembersByCommentIDs, reason: %v", err.Error())
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

	return
}
