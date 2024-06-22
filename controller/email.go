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
