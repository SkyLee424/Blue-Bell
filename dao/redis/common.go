package redis

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// keys
// 规范：
// Key + KeyName + Type + (PF)前缀
const (
	// token
	KeyAccessTokenStringPF  = "bluebell:token:access_token:"  // parma: user_id, val: access_token
	KeyRefreshTokenStringPF = "bluebell:token:refresh_token:" // parma: user_id, val: refresh_token

	// post
	KeyPostTimeZset        = "bluebell:post:time"       // member: post_id, score: time
	KeyPostScoreZset       = "bluebell:post:score"      // member: post_id, score: score
	KeyPostCommunityZsetPF = "bluebell:post:community:" // member: post_id, score: 0
	KeyPostVotedZsetPF     = "bluebell:post:voted:"     // parma: post_id, member: user_id, score: opinion
	KeyCachePF             = "bluebell:cache:"
)

var Nil = redis.Nil

// common method
func set(key string, val any, expireDuration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.Set(ctx, key, val, expireDuration)
	return errors.Wrap(cmd.Err(), "")
}

func get(key string) *redis.StringCmd {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	return rdb.Get(ctx, key)
}
