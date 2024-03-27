package rebuild

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/logger"
	"fmt"

	"github.com/pkg/errors"
)

// 如果成功重建，返回 otype_oid 下的所有 comment_id（可以直接使用这个 comment_ids，避免再读一次缓存）
func RebuildCommentIndex(objType int8, objId int64) error {
	key := fmt.Sprintf("%v%v_%v", redis.KeyCommentIndexZSetPF, objType, objId)
	exist, err := redis.Exists(key)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentIndex: Exists")
	}
	if exist { // 不需要重建
		return nil
	}

	commentIDs, err := mysql.SelectRootCommentIDs(nil, objType, objId)

	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectSubCommentIDs")
	}
	if len(commentIDs) == 0 { // 该主题下没有评论
		return nil
	}

	floors, err := mysql.SelectFloorsByCommentIDs(nil, commentIDs)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentIndex: SelectFloorsByCommentIDs")
	}
	err = redis.AddCommentIndexMembers(objType, objId, commentIDs, floors)
	if err != nil {
		return errors.Wrap(err, "rebuild:RebuildCommentIndex: AddCommentIndexMembers")
	}

	logger.Infof("rebuild:RebuildCommentIndex: Rebuild 1 data from mysql to redis")
	return nil
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
