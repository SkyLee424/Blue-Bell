package rebuild

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/logger"
	"fmt"

	"github.com/pkg/errors"
)

func RebuildCommentLikeOrHateSet(commentID, userID, objID int64, objType int8, like bool) (bool, error) {
	// 读 db，判断该用户是否给该评论点过赞
	exist, err := mysql.CheckCidUidIfExist(nil, commentID, userID, like)
	if err != nil {
		return false, errors.Wrap(err, "rebuild:RebuildCommentLikeOrHateSet: CheckCidUidIfExist")
	}
	if exist {
		err := redis.AddCommentLikeOrHateUser(commentID, userID, objID, objType, like)
		if err != nil {
			return false, errors.Wrap(err, "rebuild:RebuildCommentLikeOrHateSet: AddCommentLikeOrHateUser")
		}
		logger.Infof("rebuild:RebuildCommentLikeOrHateSet: Rebuild 1 data from mysql to redis")
	}
	return exist, nil
}

// 如果成功重建，返回 otype_oid 下的所有 comment_id（可以直接使用这个 comment_ids，避免再读一次缓存）
func RebuildCommentIndex(objType int8, objId, root int64) ([]int64, error) {
	key := fmt.Sprintf("%v%v_%v", redis.KeyCommentIndexZSetPF, objType, objId)
	exist, err := redis.Exists(key)
	if err != nil {
		return nil, errors.Wrap(err, "rebuild:RebuildCommentIndex: Exists")
	}
	if exist { // 不需要重建
		return nil, nil
	}

	var commentIDs []int64
	if root == 0 {
		commentIDs, err = mysql.SelectRootCommentIDs(nil, objType, objId)
	} else {
		commentIDs, err = mysql.SelectSubCommentIDs(nil, root)
	}
	if err != nil {
		return nil, errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectSubCommentIDs")
	}
	if len(commentIDs) == 0 { // 该主题下没有评论
		return nil, nil
	}

	floors, err := mysql.SelectFloorsByCommentIDs(nil, commentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectFloorsByCommentIDs")
	}
	err = redis.AddCommentIndexMembers(objType, objId, commentIDs, floors)
	if err != nil {
		return nil, errors.Wrap(err, "rebuild:RebuildCommentIndex: AddCommentIndexMembers")
	}

	logger.Infof("rebuild:RebuildCommentIndex: Rebuild 1 data from mysql to redis")
	return commentIDs, nil
}

func RebuildCommentContent(commentIDs []int64) error {
	logger.Debugf("Called RebuildCommentContent")
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

func RebuildCommentUserLikeOrHateMapping(userID, objID int64, objType int8, like bool) ([]int64, bool, error) {
	key := redis.KeyCommentUserLikeIDsPF
	if !like {
		key = redis.KeyCommentUserHateIDsPF
	}
	key = fmt.Sprintf("%s%d_%d_%d", key, userID, objID, objType)
	exist, err := redis.Exists(key)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentUserLikeOrHateMapping: Exists")
	}
	if exist { // 不需要重建
		// 重置 TTL
		var err error
		if like {
			err = redis.RestoreKeyExpireTime(key, redis.CommentUserLikeExpireTime)
		} else {
			err = redis.RestoreKeyExpireTime(key, redis.CommentUserHateExpireTime)
		}
		if err != nil {
			logger.Warnf("rebuild:RebuildCommentUserLikeOrHateMapping: RestoreKeyExpireTime failed")
		}
		return nil, false, nil
	}

	list, err := mysql.SelectCommentUserLikeOrHateList(nil, userID, objID, objType, like)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentUserLikeOrHateMapping: SelectCommentUserLikeOrHateList")
	}

	// 需要移除在 bluebell:comment:rem:cid 中的 comment_id
	tmp := make([]any, len(list))
	for idx, commentID := range list {
		tmp[idx] = commentID
	}
	exists, err := redis.SetIsMembers(redis.KeyCommentRemCidSet, tmp)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentUserLikeOrHateMapping: SetIsMembers")
	}
	if len(list) != len(exists) {
		return nil, false, errors.Wrap(bluebell.ErrInternal, "rebuild:RebuildCommentUserLikeOrHateMapping: list and exists' length not equal")
	}
	res := make([]int64, 0, len(list))
	res = append(res, -1) // 添加一个冗余数据到 redis，防止缓存穿透
	for idx, commentID := range list {
		if !exists[idx] {
			res = append(res, commentID)
		}
	}

	err = redis.AddCommentUserLikeOrHateMappingByCommentIDs(userID, objID, objType, like, res)
	if err != nil {
		return nil, false, errors.Wrap(err, "rebuild:RebuildCommentUserLikeOrHateMapping: AddCommentUserLikeOrHateMappingByCommentIDs")
	}
	logger.Infof("rebuild:RebuildCommentUserLikeOrHateMapping: Rebuild %d data from mysql to redis", len(res))
	return res, true, nil
}
