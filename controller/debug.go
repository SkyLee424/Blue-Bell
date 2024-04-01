package controller

import (
	common "bluebell/controller/Common"
	"bluebell/dao/localcache"
	"bluebell/logger"

	"github.com/gin-gonic/gin"
)

type ResponseForLocalCache struct {
	LocalCache      any `json:"local_cache"`
	ViewCache       any `json:"view_cache"`
	CreateTimeCache any `json:"create_time_cache"`
}

// DebugLocalCacheAllHandler 获取本地缓存接口
//
//	@Summary		获取本地缓存接口
//	@Description	获取本地缓存接口
//	@Tags			调试相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=ResponseForLocalCache}
//	@Router			/debug/localcache/all [get]
func DebugLocalCacheAllHandler(ctx *gin.Context) {
	lc := localcache.GetLocalCache()
	vc := localcache.GetViewCache()
	cc := localcache.GetCreateTimeCache()

	m0 := lc.GetALL(false)
	m1 := vc.GetALL(false)
	m2 := cc.GetALL(false)

	res := &ResponseForLocalCache{
		LocalCache:      m0,
		ViewCache:       m1,
		CreateTimeCache: m2,
	}
	logger.Debugf("%v", res)
	common.ResponseSuccess(ctx, res)
}

// EmptyHandler0 空接口
//
//	@Summary		空接口
//	@Description	空接口，测试 gin 框架基础性能使用
//	@Tags			调试相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{}
//	@Router			/empty0 [get]
func EmptyHandler0(ctx *gin.Context)  {
	common.ResponseSuccess(ctx, nil)
}

// EmptyHandler1 空接口
//
//	@Summary		空接口
//	@Description	空接口，但有鉴权，测试 gin 框架基础性能使用
//	@Tags			调试相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{}
//	@Router			/empty1 [get]
func EmptyHandler1(ctx *gin.Context)  {
	userID, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.UserUpdateHandler: get user_id from context failed")
		return
	}

	common.ResponseSuccess(ctx, userID)
}
