package redis

import (
	"bluebell/algorithm"
	bluebell "bluebell/errors"
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func SetPost(postID, communityID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout*2)
	defer cancel()

	curTimeStamp := time.Now().Unix()

	pipeline := rdb.TxPipeline()
	// 缓存 KeyPostTimeZset
	// 这里可能存在问题：
	// 没有设置 TTL，虽然是懒删除（查询 redis 发现过期才删除）
	// 但如果一直不查询，就一直不删除，存在占用内存的情况

	// 解决方案：可以设置定时任务，定期检查 redis 中 "过期" 的 key，将其持久化到 mysql

	pipeline.ZAdd(ctx, KeyPostTimeZset, redis.Z{
		Member: postID,
		Score:  float64(curTimeStamp),
	})

	// 缓存 KeyPostScoreZset（curTimeStamp）
	pipeline.ZAdd(ctx, KeyPostScoreZset, redis.Z{
		Member: postID,
		Score:  algorithm.GetPostScoreByReddit(time.Now().Unix(), 1), // 使用 reddit 投票算法
	})

	// 缓存 KeyPost
	pipeline.ZAdd(ctx, KeyPostCommunityZsetPF+strconv.FormatInt(communityID, 10), redis.Z{
		Member: postID,
		Score:  float64(curTimeStamp),
	})

	// 简单的错误处理
	// 注意，如果有其它客户端并发修改 key，事务会失败
	// 后续添加重试逻辑
	_, err := pipeline.Exec(ctx)
	return err

}

func GetPostPublishTimeByPostID(post_id string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.ZScore(ctx, KeyPostTimeZset, post_id)
	if cmd.Err() != nil {
		return 0, errors.Wrap(cmd.Err(), "get post publish time")
	}
	return cmd.Val(), nil
}

func GetUserPostDirection(post_id, user_id int64) (int8, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.ZScore(ctx, KeyPostVotedZsetPF+strconv.FormatInt(post_id, 10), strconv.FormatInt(user_id, 10))
	if cmd.Err() != nil {
		// 默认返回 0 值(包含 redis.Nil)
		return 0, errors.Wrap(cmd.Err(), "get user post direction")
	}
	return int8(cmd.Val()), nil
}

func SetPostScore(post_id int64, score float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.ZAdd(ctx, KeyPostScoreZset, redis.Z{
		Member: post_id,
		Score:  score,
	})
	if cmd.Err() != nil {
		return errors.Wrap(cmd.Err(), "set post score")
	}
	return nil
}

func SetUserPostDirection(post_id, user_id int64, direction int8) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.ZAdd(ctx, KeyPostVotedZsetPF+strconv.FormatInt(post_id, 10), redis.Z{
		Member: user_id,
		Score:  float64(direction),
	})
	if cmd.Err() != nil {
		return errors.Wrap(cmd.Err(), "set user post direction")
	}
	return nil
}

func GetPostIDs(pageNum, pageSize int64, orderBy string) ([]string, int, error) {
	var key string
	if orderBy == "time" {
		key = KeyPostTimeZset
	} else if orderBy == "score" {
		key = KeyPostScoreZset
	} else if orderBy == "views" {
		key = KeyPostViewsZset
	} else {
		return nil, 0, bluebell.ErrInvalidParam
	}

	return getPostIDHelper(key, pageNum, pageSize)
}

func GetPostIDsByCommunity(pageNum, pageSize int64, orderBy string, communityID int64) ([]string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	var oKey string // orderby key
	if orderBy == "time" {
		oKey = KeyPostTimeZset
	} else {
		oKey = KeyPostScoreZset
	}

	cKey := KeyPostCommunityZsetPF + strconv.FormatInt(communityID, 10)
	key := KeyCachePF + "post_orderby:" + orderBy + ":" + strconv.FormatInt(communityID, 10)

	// 求交集很重，建立缓存以优化性能
	// 不存在缓存，建立缓存
	if rdb.Exists(ctx, key).Val() < 1 {
		pipe := rdb.Pipeline()
		pipe.ZInterStore(ctx, key, &redis.ZStore{
			Aggregate: "MAX", // 指定合并规则
			Keys:      []string{oKey, cKey},
		})
		tls := viper.GetInt("redis.cache_key_tls") // 读取配置文件
		pipe.Expire(ctx, key, time.Duration(tls)*time.Second)
		_, err := pipe.Exec(ctx)
		if err != nil {
			return nil, 0, errors.Wrap(err, "build cache")
		}
	} else {
		// 在过期前再次访问，可能是热点 key，重置 TTL
		tls := viper.GetInt("hot_key_tls") // 读取配置文件
		rdb.Expire(ctx, key, time.Duration(tls)*time.Second)
	}

	return getPostIDHelper(key, pageNum, pageSize)
}

func GetPostVoteNum(postID string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.ZCount(ctx, KeyPostVotedZsetPF+postID, "1", "1")
	if err := cmd.Err(); err != nil {
		return 0, errors.Wrap(err, "get post vote nums")
	}
	return cmd.Val(), nil
}

func GetPostUpVoteNums(postIDs []string) ([]int64, error) {
	return getPostVoteNumHelper(postIDs, "1")
}

func GetPostDownVoteNums(postIDs []string) ([]int64, error) {
	return getPostVoteNumHelper(postIDs, "-1")
}

func getPostVoteNumHelper(postIDs []string, opinion string) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for _, postID := range postIDs {
		pipe.ZCount(ctx, KeyPostVotedZsetPF+postID, opinion, opinion)
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get post vote nums(pipelined)")
	}

	voteNums := make([]int64, 0, len(postIDs))
	for _, cmd := range cmds {
		voteNum := cmd.(*redis.IntCmd).Val()
		voteNums = append(voteNums, voteNum)
	}

	return voteNums, nil
}

func GetPostScore(post_id int64) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.ZScore(ctx, KeyPostScoreZset, strconv.FormatInt(post_id, 10))
	if cmd.Err() != nil {
		// 默认返回 0 值(包含 redis.Nil)
		return 0, errors.Wrap(cmd.Err(), "get post score")
	}
	return cmd.Val(), nil
}

func GetPostScores(postIDs []string) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for _, postID := range postIDs {
		pipe.ZScore(ctx, KeyPostScoreZset, postID)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get post scores(pipelined)")
	}

	postScores := make([]int64, 0, len(postIDs))
	for _, cmd := range cmds {
		postScore := cmd.(*redis.FloatCmd).Val()
		postScores = append(postScores, int64(postScore))
	}

	return postScores, nil
}

// 获取过期帖子的 ID
func GetExpiredPostID(targetTimeStamp int64) ([]string, error) {
	// postIDs 是按照发布时间降序排序的
	postIDs, _, err := getPostIDHelper(KeyPostTimeZset, 1, (1 << 62))
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	// 二分查找第一个小于等于 targetTimeStamp 的 postID 下标
	L := 0
	R := len(postIDs) - 1
	pos := -1
	for L <= R {
		mid := L + (R-L)/2
		postTime, err := GetPostPublishTimeByPostID(postIDs[mid])
		if err != nil {
			return nil, errors.Wrap(err, "get post publish time")
		}
		if postTime <= float64(targetTimeStamp) {
			pos = mid
			R = mid - 1
		} else {
			L = mid + 1
		}
	}

	if pos == -1 {
		return nil, nil
	}
	return postIDs[pos:], nil
}

func DeletePostScores(postIDs []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.ZRem(ctx, KeyPostScoreZset, postIDs)
	return errors.Wrap(cmd.Err(), "DeletePostScores")
}

func DeletePostTimes(postIDs []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cmd := rdb.ZRem(ctx, KeyPostTimeZset, postIDs)
	return errors.Wrap(cmd.Err(), "DeletePostTimes")
}

func DeletePostVotedNums(postIDs []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	pipe := rdb.Pipeline()
	for _, postID := range postIDs {
		pipe.Del(ctx, KeyPostVotedZsetPF+postID)
	}
	_, err := pipe.Exec(ctx)
	return errors.Wrap(err, "DeletePostVotedNums")
}

func DeleteExpiredPostInCommunity(communityID string, targetTimeStamp int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := KeyPostCommunityZsetPF + communityID
	cmd := rdb.ZRange(ctx, key, 0, (1 << 62))
	if cmd.Err() != nil && !errors.Is(cmd.Err(), redis.Nil) {
		return errors.Wrap(cmd.Err(), "DeletePostInCommunity: get post ids")
	}

	postIDs := cmd.Val() // 按发布时间升序排序

	// 二分查找第一个大于 targetTimeStamp 的 postID 下标
	L := 0
	R := len(postIDs) - 1
	pos := R + 1 // 注意，默认情况假设所有都过期！
	for L <= R {
		mid := L + (R-L)/2
		cmd := rdb.ZScore(ctx, key, postIDs[mid])
		if cmd.Err() != nil {
			return errors.Wrap(cmd.Err(), "DeletePostInCommunity: get post publish time")
		}
		postTime := cmd.Val()
		if postTime > float64(targetTimeStamp) {
			pos = mid
			R = mid - 1
		} else {
			L = mid + 1
		}
	}

	pipe := rdb.Pipeline()
	for i := 0; i < pos; i++ { // 删除过期帖子
		pipe.ZRem(ctx, key, postIDs[i])
	}

	_, err := pipe.Exec(ctx)
	return errors.Wrap(err, "DeletePostInCommunity: delete post in community")
}

func getPostIDHelper(key string, pageNum, pageSize int64) ([]string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	start := (pageNum - 1) * pageSize
	stop := start + pageSize - 1

	cmd := rdb.ZRevRange(ctx, key, start, stop)
	if cmd.Err() != nil {
		return nil, 0, errors.Wrap(cmd.Err(), "get post ids")
	}

	cmd1 := rdb.ZCard(ctx, key)

	return cmd.Val(), int(cmd1.Val()), errors.Wrap(cmd1.Err(), "redis:getPostIDHelper: ZCard")
}

// func GetAgreeNum(post_id int64) (int64, error) {
// 	return getAgreeNumHelper(post_id, "1")
// }

// func GetDisagreeNum(post_id int64) (int64, error) {
// 	return getAgreeNumHelper(post_id, "-1")
// }

// func getAgreeNumHelper(post_id int64, opinion string) (int64, error) {
// ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
// defer cancel()
// 	cmd := rdb.ZCount(ctx, KeyPostVotedZsetPF+strconv.FormatInt(post_id, 10), opinion, opinion)
// 	return cmd.Val(), cmd.Err()
// }
