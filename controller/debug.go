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
