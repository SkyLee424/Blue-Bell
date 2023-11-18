package workers

import (
	"bluebell/dao/redis"
	"bluebell/logger"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// 检查错误，如果有错误：
//
// 1. 输出日志
// 2. 修改 waitTime 为较小值，尽快重试
func checkError(err error, waitTime *time.Duration, wg *sync.WaitGroup) bool {
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.ErrorWithStack(err)
		*waitTime = time.Second * 10 // 10 s 后再次尝试获取
		wg.Done()
		return false
	}
	return true
}

func getExpiredKeys(keys []string, expiredTime time.Duration) ([]string, error) {
	expiredKeys := make([]string, 0, len(keys)) // 避免扩容带来的开销
	idleTimes, err := redis.GetKeysIdleTime(keys)
	if err != nil {
		return nil, errors.Wrap(err, "workers:common:getExpiredKeys: GetKeysIdleTime")
	}
	for i := 0; i < len(keys); i++ {
		if idleTimes[i] > expiredTime {
			expiredKeys = append(expiredKeys, keys[i])
		}
	}
	return expiredKeys, nil
}
