package controller

import (
	common "bluebell/controller/Common"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"

	bluebell "bluebell/errors"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// UserRegisterHandler 用户注册接口
//
//	@Summary		用户注册接口
//	@Description	用户注册接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			user_info	body		models.ParamUserRegist	false	"用户信息（包含用户名、密码、重复密码）"
//	@Success		200			{object}	common.Response{data=common.ResponseTokens}
//	@Router			/user/register [post]
func UserRegisterHandler(ctx *gin.Context) {
	// 数据解析
	var usr models.ParamUserRegist

	// 使用 validator 在解析数据的同时做参数校验
	if err := ctx.ShouldBindJSON(&usr); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	// 注册
	access_token, refresh_token, err := logic.Regist(&models.User{UserName: usr.Username, Password: usr.Password})
	if err != nil {
		if errors.Is(err, bluebell.ErrUserExist) {
			common.ResponseError(ctx, common.CodeUserExist)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			// 打日志
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, common.ResponseTokens{
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	})
}

// UserRegisterHandler 用户登录接口
//
//	@Summary		用户登录接口
//	@Description	用户登录接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			usernameANDpassword	body		models.ParamUserLogin	false	"用户信息（包含用户名、密码）"
//	@Success		200					{object}	common.Response{data=common.ResponseUserLogin}
//	@Router			/user/login [post]
func UserLoginHandler(ctx *gin.Context) {
	// 解析、校验数据
	var usr models.User
	if err := ctx.ShouldBindJSON(&usr); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	// 登录
	access_token, refresh_token, err := logic.Login(&usr)
	if err != nil {
		if errors.Is(err, bluebell.ErrUserNotExist) {
			common.ResponseError(ctx, common.CodeUserNotExist)
		} else if errors.Is(err, bluebell.ErrWrongPassword) {
			common.ResponseError(ctx, common.CodeWrongPassword)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			// 打日志
			logger.ErrorWithStack(err)
		}
		return
	}

	// 响应
	common.ResponseSuccess(ctx, common.ResponseUserLogin{
		UserName: usr.UserName,
		UserID: usr.UserID,
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	})
}

// login: gen aToken and rToken, refresh redis token([key, val]: [user_id, aToken])

// get resource: parse aToken, judge valid

// if valid:
// judge aToken :
// 1. aToken = rdb.aToken, ok
// 2. aToken != rdb.aToken, not ok, relogin

// if not valid:
// return, front end will ask for refresh access token
