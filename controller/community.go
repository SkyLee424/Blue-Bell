package controller

import (
	common "bluebell/controller/Common"
	bluebell "bluebell/errors"
	"bluebell/logger"
	"bluebell/logic"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// CommunityListHandler 社区列表接口
//
//	@Summary		社区列表接口
//	@Description	社区列表接口
//	@Tags			社区相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.CommunityDTO}
//	@Router			/community/list [get]
func CommunityListHandler(ctx *gin.Context) {
	list, err := logic.GetCommunityList()
	if err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}
	common.ResponseSuccess(ctx, list)
}

// CommunityDetailHandler 社区详情接口
//
//	@Summary		社区详情接口
//	@Description	给定社区 id，获取社区详情
//	@Tags			社区相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Param			community_id	query	integer false	"社区 id"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=models.CommunityDTO}
//	@Router			/community/detail [get]
func CommunityDetailHandler(ctx *gin.Context) {
	// 解析参数
	community_id, exists := ctx.GetQuery("community_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInvalidParam)
		return
	}
	id, err := strconv.ParseInt(community_id, 10, 64)
	if err != nil {
		common.ResponseError(ctx, common.CodeInvalidParam)
		return
	}

	// 查询
	detail, err := logic.GetCommunityDetailByID(id)
	if err != nil {
		if errors.Is(err, bluebell.ErrNoSuchCommunity) {
			common.ResponseError(ctx, common.CodeNoSuchCommunity)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	// 返回
	common.ResponseSuccess(ctx, detail)
}
