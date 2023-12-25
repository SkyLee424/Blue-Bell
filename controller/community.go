package controller

import (
	common "bluebell/controller/Common"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"
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

// CommunityCreateHandler 创建社区接口（root user only）
//
//	@Summary		创建社区接口（root user only）
//	@Description	创建社区接口（root user only）
//	@Tags			社区相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string						false	"Bearer 用户令牌"
//	@Param			object			body	models.ParamCommunityCreate	false	"社区详细信息"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/community/create [post]
func CommunityCreateHandler(ctx *gin.Context) {
	// 获取 user_id
	value, _ := ctx.Get("user_id")
	userID, ok := value.(int64)
	if !ok {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.CreatePostHandler: convert user_id from context to int64 failed")
		return
	}
	if userID != 0 {
		common.ResponseError(ctx, common.CodeForbidden)
		return
	}

	params := new(models.ParamCommunityCreate)
	if err := ctx.ShouldBindJSON(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}

	if err := logic.CreateCommunity(params); err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	common.ResponseSuccess(ctx, nil)
}
