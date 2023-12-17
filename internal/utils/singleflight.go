package utils

import (
	bluebell "bluebell/errors"
	"context"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

func SfDoWithTimeout(sfGrp *singleflight.Group, key string, timeout, interval time.Duration, fn func() (any, error)) (v any, err error) {
	// 获取 res 通道
	ch := sfGrp.DoChan(key, fn)

	// 定时 forget，防止全部返回错误，以及数据不一致性问题
	go func() {
		time.Sleep(interval)
		sfGrp.Forget(key)
	}()

	// 超时控制
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case res := <-ch:
		return res.Val, errors.Wrap(res.Err, "utils:SfDoWithTimeout: fn")
	case <-ctx.Done():
		return nil, errors.Wrap(bluebell.ErrTimeout, "utils:SfDoWithTimeout: fn")
	}
}
