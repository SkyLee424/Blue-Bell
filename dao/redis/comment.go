package redis

import (
	bluebell "bluebell/errors"
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

/* bluebell:comment:index: */
func AddCommentIndexMembers(objType int8, objID int64, commentIDs []int64, floor []int) error {
	if len(commentIDs) != len(floor) {
		return errors.Wrap(bluebell.ErrInternal, "redis:AddCommentIndexMember: commentIDs and floors length not equal")
	}
	key := fmt.Sprintf("%v%v_%v", KeyCommentIndexZSetPF, objType, objID)
	
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for i := 0; i < len(commentIDs); i++ {
		pipe.ZAdd(ctx, key, redis.Z{
			Member: commentIDs[i],
			Score: float64(floor[i]),
		})
	}
	_, err := pipe.Exec(ctx)

	return errors.Wrap(err, "redis:AddCommentIndexMember: ZAdd")
}

func GetCommentIndexMember(objType int8, objID int64) ([]int64, error)  {
	key := fmt.Sprintf("%v%v_%v", KeyCommentIndexZSetPF, objType, objID)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.ZRange(ctx, key, 0, (1 << 62))
	if cmd.Err() != nil {
		return nil, errors.Wrap(cmd.Err(), "redis:GetCommentIndexMember: ZRange")
	}
	commentIDStrs := cmd.Val()
	commentIDs := make([]int64, len(commentIDStrs))
	for i := 0; i < len(commentIDStrs); i++ {
		commentIDs[i], _ = strconv.ParseInt(commentIDStrs[i], 10, 64)
	}
	return commentIDs, nil
}

func RemCommentIndexMembersByCommentID(objType int8, objID int64, commentID int64) error {
	key := fmt.Sprintf("%v%v_%v", KeyCommentIndexZSetPF, objType, objID)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	
	cmd := rdb.ZRem(ctx, key, commentID)
	return errors.Wrap(cmd.Err(), "redis:RemCommentIndexMembersByCommentIDs: SRem")
}

/* bluebell:comment:content: */
func AddCommentContents(commentIDs []int64, content []string) error {
	if len(commentIDs) != len(content) {
		return errors.Wrap(bluebell.ErrInternal, "redis:AddCommentContents: commentIDs and content length not equal")
	}
	keys := make([]string, len(commentIDs))
	for i := 0; i < len(keys); i++ {
		keys[i] = fmt.Sprintf("%v%v", KeyCommentContentStringPF, commentIDs[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for i := 0; i < len(keys); i++ {
		pipe.Set(ctx, keys[i], content[i], -1)
	}
	_, err := pipe.Exec(ctx)

	return errors.Wrap(err, "redis:AddCommentContent: Set")
}

func GetCommentContents(commentIDs []int64) ([]string, error) {
	keys := make([]string, len(commentIDs))
	for i := 0; i < len(keys); i++ {
		keys[i] = fmt.Sprintf("%v%v", KeyCommentContentStringPF, commentIDs[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for i := 0; i < len(keys); i++ {
		pipe.Get(ctx, keys[i])
	}
	cmds, err := pipe.Exec(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "redis:GetCommentContents: Get")
	}

	content := make([]string, len(cmds))
	for i := 0; i < len(cmds); i++ {
		cmd := cmds[i].(*redis.StringCmd)
		content[i] = cmd.Val()
	}

	return content, nil
}

func DelCommentContentsByCommentIDs(commentIDs []int64) error {
	keys := make([]string, len(commentIDs))
	for i := 0; i < len(keys); i++ {
		keys[i] = fmt.Sprintf("%v%v", KeyCommentContentStringPF, commentIDs[i])
	}

	return DelKeys(keys)
}

/* bluebell:comment:likeset: */
func CheckCommentLikeOrHateIfExistUser(commentID, userID int64, like bool) (bool, error) {
	key := getCommentLikeOrHateSetKey(commentID, like)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	
	cmd := rdb.SIsMember(ctx, key, userID)
	return cmd.Val(), errors.Wrap(cmd.Err(), "redis:CheckCommentLikeOrHateIfExistUser: SIsMember")
}

func AddCommentLikeOrHateUser(commentID, userID int64, like bool) error {
	key := getCommentLikeOrHateSetKey(commentID, like)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.SAdd(ctx, key, userID)

	return errors.Wrap(cmd.Err(), "redis:AddCommentLikeOrHateUser: SAdd")
}

func RemCommentLikeOrHateUser(commentID int64, userID int64, like bool) error {
	key := getCommentLikeOrHateSetKey(commentID, like)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.SRem(ctx, key, userID)
	return errors.Wrap(cmd.Err(), "redis:RemCommentLikeOrHateUser: SRem")
}

func DelCommentLikeOrHateUserByCommentIDs(commentIDs []int64, like bool) error {
	keys := make([]string, len(commentIDs))
	for i := 0; i < len(keys); i++ {
		keys[i] = getCommentLikeOrHateSetKey(commentIDs[i], like)
	}

	return DelKeys(keys)
}

/* bluebell:comment:rem:cid_uid */
func AddCommentRemCidUid(CidUid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.SAdd(ctx, KeyCommentRemCidUidSet, CidUid)
	return errors.Wrap(cmd.Err(), "redis:AddCommentRemCidUid: SAdd")
}

func RemCommentRemCidUid(CidUid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.SRem(ctx, KeyCommentRemCidUidSet, CidUid)
	return errors.Wrap(cmd.Err(), "redis:RemCommentRemCidUid: SRem")
}

func CheckCommentRemCidUidIfExistCidUid(CidUid string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.SIsMember(ctx, KeyCommentRemCidUidSet, CidUid)
	return cmd.Val(), errors.Wrap(cmd.Err(), "redis:CheckCommentRemCidUidIfExistCidUid: SIsMember")
}

/* bluebell:comment:like: */
func DelCommentLikeOrHateCountByCommentIDs(commentIDs []int64, like bool) error {
	keys := make([]string, 0, len(commentIDs))
	for _, commentID := range commentIDs {
		key := getCommentLikeOrHateStringKey(commentID, like)
		keys = append(keys, key)
	}

	return DelKeys(keys)
}

func IncrCommentLikeOrHateCount(commentID int64, offset int, like bool) error {
	key := getCommentLikeOrHateStringKey(commentID, like)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.IncrBy(ctx, key, int64(offset))
	return errors.Wrap(cmd.Err(), "redis:IncrCommentLikeOrHateCount: IncrBy")
}

func GetCommentLikeOrHateCount(commentID int64, like bool) (int, error) {
	key := getCommentLikeOrHateStringKey(commentID, like)

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.Get(ctx, key)
	count, _ := strconv.ParseInt(cmd.Val(), 10, 32)
	return int(count), errors.Wrap(cmd.Err(), "redis:GetCommentLikeOrHateCount: Get")
}

func GetCommentLikeOrHateCountByCommentIDs(commentIDs []int64, like bool) ([]int, error) {
	keys := make([]string, 0, len(commentIDs))
	for _, commentID := range commentIDs {
		key := getCommentLikeOrHateStringKey(commentID, like)
		keys = append(keys, key)
	}

	return GetCommentLikeOrHateCountByKeys(keys)
}

func GetCommentLikeOrHateCountByKeys(keys []string) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for _, key := range keys {
		pipe.Get(ctx, key)
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errors.Wrap(err, "redis:GetCommentLikeOrHateCountByCommentIDs: Get")
	}
	counts := make([]int, 0, len(keys))
	for _, cmd := range cmds {
		count, _ := strconv.ParseInt(cmd.(*redis.StringCmd).Val(), 10, 32)
		counts = append(counts, int(count))
	}
	return counts, nil
}

func getCommentLikeOrHateSetKey(commentID int64, like bool) string {
	pf := KeyCommentLikeSetPF
	if !like {
		pf = KeyCommentHateSetPF
	}
	return pf + strconv.FormatInt(commentID, 10)
}

func getCommentLikeOrHateStringKey(commentID int64, like bool) string {
	pf := KeyCommentLikeStringPF
	if !like {
		pf = KeyCommentHateStringPF
	}
	return fmt.Sprintf("%s%v", pf, commentID)
}
