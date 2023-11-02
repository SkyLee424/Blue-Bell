package controller

import (
	common "bluebell/controller/Common"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// RefreshTokenHandler 刷新 access_token 接口
//
//	@Summary		刷新 access_token 接口
//	@Description	根据 Bearer Authorization 中携带的 refresh_token，刷新 access_token
//	@Tags			Token 相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"refresh_token"
//	@Param			access_token	query	string	false	"旧的 access_token"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=common.ResponseTokens}
//	@Router			/token/refresh [get]
func RefreshTokenHandler(ctx *gin.Context) {
	// 解析数据
	header := ctx.Request.Header.Get("Authorization")
	parts := strings.Split(header, " ")
	aTokenStr := ctx.Query("access_token")

	// 获取新的 access_token
	access_token, err := logic.RefreshToken(parts[1], aTokenStr)
	if err != nil {
		if errors.Is(err, bluebell.ErrExpiredToken) {
			common.ResponseError(ctx, common.CodeExpiredLogin)
		} else if errors.Is(err, bluebell.ErrInvalidToken) {
			common.ResponseError(ctx, common.CodeInvalidToken)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	UserID, _ := utils.ParseToken(access_token)

	// 更新 redis 中的 access_token
	if err := logic.SetUserAccessToken(UserID, access_token, utils.GetAccessTokenExpireDuration()); err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	common.ResponseSuccess(ctx, gin.H{
		"access_token": access_token,
	})
}
