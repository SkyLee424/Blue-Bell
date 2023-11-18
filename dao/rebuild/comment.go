package rebuild

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	"bluebell/logger"
	"fmt"

	"github.com/pkg/errors"
)

func RebuildCommentLikeOrHateSet(commentID, userID int64, like bool) (bool, error) {
	CidUid := fmt.Sprintf("%v:%v", commentID, userID)

	// 读 db，判断该用户是否给该评论点过赞
	exist, err := mysql.CheckCidUidIfExist(nil, CidUid, like)
	if err != nil {
		return false, errors.Wrap(err, "rebuild:RebuildCommentLikeOrHateSet: CheckCidUidIfExist")
	}
	if exist {
		err := redis.AddCommentLikeOrHateUser(commentID, userID, like)
		if err != nil {
			return false, errors.Wrap(err, "rebuild:RebuildCommentLikeOrHateSet: AddCommentLikeOrHateUser")
		}
		logger.Infof("rebuild:RebuildCommentLikeOrHateSet: Rebuild 1 data from mysql to redis")
	}
	return exist, nil
}

// 返回参数：commentID、是否重建、错误
// 如果成功重建，返回 otype_oid 下的所有 comment_id（可以直接使用这个 comment_ids，避免再读一次缓存）
func RebuildCommentIndex(objType int8, objId, root int64) ([]int64, bool, error) {
	key := fmt.Sprintf("%v%v_%v", redis.KeyCommentIndexZSetPF, objType, objId)
	exist, err := redis.Exists(key)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentIndex: Exists")
	}
	if exist { // 不需要重建
		return nil, false, nil
	}

	var commentIDs []int64
	if root == 0 {
		commentIDs, err = mysql.SelectRootCommentIDs(nil, objType, objId)
	} else {
		commentIDs, err = mysql.SelectSubCommentIDs(nil, root)
	}
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectSubCommentIDs")
	}
	if len(commentIDs) == 0 { // 该主题下没有评论
		return nil, true, nil
	}

	floors, err := mysql.SelectFloorsByCommentIDs(nil, commentIDs)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectFloorsByCommentIDs")
	}
	err = redis.AddCommentIndexMembers(objType, objId, commentIDs, floors)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentIndex: AddCommentIndexMembers")
	}

	logger.Infof("rebuild:RebuildCommentIndex: Rebuild 1 data from mysql to redis")
	return commentIDs, true, nil
}

func RebuildCommentContent(commentIDs []int64) error {
	keys := make([]string, len(commentIDs))
	for i := 0; i < len(commentIDs); i++ {
		keys[i] = fmt.Sprintf("%v%v", redis.KeyCommentContentStringPF, commentIDs[i])
	}
	exists, err := redis.ExistsKeys(keys)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentContent: ExistKeys")
	}

	missCommentIDs := make([]int64, 0, len(commentIDs)) // miss 的 commentID
	for i := 0; i < len(exists); i++ {
		if !exists[i] {
			missCommentIDs = append(missCommentIDs, commentIDs[i])
		}
	}

	if len(missCommentIDs) == 0 { // 不需要重建
		return nil
	}

	// 重建
	content, err := mysql.SelectCommentContentByCommentIDs(nil, missCommentIDs)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentContent: SelectCommentContentByCommentIDs")
	}
	message := make([]string, len(content))
	for i := 0; i < len(content); i++ {
		message[i] = content[i].Message
	}
	err = redis.AddCommentContents(missCommentIDs, message)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentContent: AddCommentContents")
	}

	logger.Infof("rebuild:RebuildCommentContent: Rebuild %d data from mysql to redis", len(missCommentIDs))
	return nil
}
