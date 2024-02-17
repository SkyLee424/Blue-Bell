package controller

import (
	common "bluebell/controller/Common"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"

	"github.com/gin-gonic/gin"
)

// EmailSendVerificationCodeHandler 发送邮箱验证码接口
//
//	@Summary		发送邮箱验证码接口
//	@Description	给用户发送邮箱验证码的接口
//	@Tags			邮箱相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamSendEmailVerificationCode	false "查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/email/verification [post]
func EmailSendVerificationCodeHandler(ctx *gin.Context) {
	params := models.ParamSendEmailVerificationCode{}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}
	if err := logic.SendEmailVerificationCode(params); err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	common.ResponseSuccess(ctx, nil)
}

// EmailSendVerificationCodeHandler 验证邮箱验证码接口
//
//	@Summary		验证邮箱验证码接口
//	@Description	验证用户输入的邮箱验证码与后端是否一致的接口
//	@Tags			邮箱相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamVerifyEmailVerificationCode	false "查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/email/verification [get]
func EmailGetVerificationCodeHandler(ctx *gin.Context) {
	params := models.ParamVerifyEmailVerificationCode{}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}
	code, err := logic.GetEmailVerificationCode(params)
	if err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}
	if code != params.Code {
		common.ResponseError(ctx, common.CodeInvalidVerificationCode)
		return
	}
	common.ResponseSuccess(ctx, nil)
}
