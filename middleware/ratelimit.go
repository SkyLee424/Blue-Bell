package middleware

import (
	controller "bluebell/controller/Common"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	ratelimit2 "go.uber.org/ratelimit"
)

// 限流中间件，如果没有可用令牌，直接拒绝请求
//
// rate：令牌生成速率，例如，rate = 0.1，代表每秒生成 0.1 * capacity 个令牌
//
// capacity：令牌桶大小
func RateLimit(rate float64, capacity int64) gin.HandlerFunc {
	bucket := ratelimit.NewBucketWithRate(rate, capacity)
	return func(ctx *gin.Context) {
		// 如果取得的令牌数量与总的令牌数不相等，说明令牌数不够，限流
		if bucket.TakeAvailable(1) != 1 {
			// 直接拒绝请求
			controller.ResponseError(ctx, controller.CodeServerBusy)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func RateLimit2(rate int) gin.HandlerFunc {
	bucket := ratelimit2.New(rate) // 每秒产生 rate 个水滴，也就是最多允许 rate 个请求
	return func(ctx *gin.Context) {
		bucket.Take() // 返回下一次滴水的时间
		ctx.Next()
	}
}


// 两个中间件使用同一个桶

var bucket = ratelimit.NewBucketWithRate(0.1, 5000)

func RateLimitForHighPriorityTask() gin.HandlerFunc  {
	return func(ctx *gin.Context) {
		bucket.Take(1) // 高优先级任务直接取，不用考虑限流，保证高可用性
		ctx.Next()
	}
}

func RateLimitForLowPriorityTask() gin.HandlerFunc  {
	return func(ctx *gin.Context) {
		if waitTime := bucket.Take(1); waitTime != 0 { // 低优先级任务取令牌，考虑限流
			// 等待
			time.Sleep(waitTime)
		} 
		ctx.Next()
	}
}