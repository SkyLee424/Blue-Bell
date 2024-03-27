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

func InitRedis() {
	// 读取配置
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port")),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
		PoolSize: viper.GetInt("redis.poolsize"), // 连接池大小
	})
	redisTimeout = time.Duration(viper.GetInt64("redis.max_oper_time")) * time.Second // 读取配置

	// ping 一下
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	cmd := rdb.Ping(ctx)
	if cmd.Err() != nil {
		panic(fmt.Sprintf("redis: %s", cmd.Err()))
	}

	// 加载 lua 脚本
	err := UploadLuaScript()
	if err != nil {
	    panic(fmt.Sprintf("redis: %v", err.Error()))
	}
}

func GetRDB() *redis.Client {
	return rdb
}
