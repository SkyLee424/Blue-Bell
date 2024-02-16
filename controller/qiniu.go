package controller

import (
	common "bluebell/controller/Common"
	"bluebell/internal/utils"
	"bluebell/logic"
	"bluebell/models"

	"github.com/gin-gonic/gin"
)

// QiniuGenUploadTokenHandler 获取七牛云文件上传 token 接口
//
//	@Summary		获取七牛云文件上传 token 接口
//	@Description	获取七牛云文件上传 token 接口
//	@Tags			七牛云相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=string}
//	@Router			/qiniu/upload/gentoken [get]
func QiniuGenUploadTokenHandler(ctx *gin.Context) {
	uploadToken := logic.QiniuGenUploadToken()
	common.ResponseSuccess(ctx, uploadToken)
}

// QiniuUploadCallbackHandler 七牛云上传完毕的回调接口
//
//	@Summary		七牛云上传完毕的回调接口
//	@Description	七牛云上传完毕的回调接口（七牛云使用）
//	@Tags			七牛云相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	body	models.QiniuCallbackBody	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=common.ResponseQiniuCallback{}}
//	@Router			/qiniu/upload/callback [post]
func QiniuUploadCallbackHandler(ctx *gin.Context) {
	params := models.QiniuCallbackBody{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	fileName, URL := logic.QiniuUploadCallback(params)
	common.ResponseSuccess(ctx, common.ResponseQiniuCallback{
		FileName: fileName,
		URL:      URL,
	})
}
