package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var rdb *redis.Client

var redisTimeout time.Duration
var CommentUserLikeExpireTime time.Duration
var CommentUserHateExpireTime time.Duration

func InitRedis() {
	// 读取配置
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port")),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
		PoolSize: viper.GetInt("redis.poolsize"), // 连接池大小
	})

	redisTimeout = time.Duration(viper.GetInt64("redis.max_oper_time")) * time.Second // 读取配置
	CommentUserLikeExpireTime = time.Duration(viper.GetInt64("service.comment.like_hate_user.like_expire_time")) * time.Second
	CommentUserHateExpireTime = time.Duration(viper.GetInt64("service.comment.like_hate_user.hate_expire_time")) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.Ping(ctx)
	if cmd.Err() != nil {
		panic(fmt.Sprintf("redis: %s", cmd.Err()))
	}
}

func GetRDB() *redis.Client {
	return rdb
}
