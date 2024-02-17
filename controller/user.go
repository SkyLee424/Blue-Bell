package controller

import (
	common "bluebell/controller/Common"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"
	"strconv"

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
	access_token, refresh_token, err := logic.UserRegist(&models.User{
		UserName: usr.Username, 
		Password: usr.Password, 
		Email: usr.Email,
		Avatar: usr.Avatar,
	})
	if err != nil {
		if errors.Is(err, bluebell.ErrUserExist) {
			common.ResponseError(ctx, common.CodeUserExist)
		} else if errors.Is(err, bluebell.ErrEmailExist) {
			common.ResponseErrorWithMsg(ctx, common.CodeUserExist, "邮箱已经被注册")
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
	var params models.ParamUserLogin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	// 登录
	usr, access_token, refresh_token, err := logic.UserLogin(&params)
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
		Avatar: usr.Avatar,
		Email: usr.Email,
		Gender: usr.Gender,
		Intro: usr.Intro,
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	})
}

// UserInfoHandler 用户信息获取接口
//
//	@Summary		获取用户信息的接口
//	@Description	根据 token 获取用户信息的接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=models.UserDTO}
//	@Router			/user/info [get]
func UserInfoHandler(ctx *gin.Context)  {
	userID, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.UserUpdateHandler: get user_id from context failed")
		return
	}

	userInfo, err := logic.UserGetInfo(userID.(int64))
	if err != nil {
		if errors.Is(err, bluebell.ErrUserNotExist) {
			common.ResponseError(ctx, common.CodeUserNotExist)
		} else {
			logger.ErrorWithStack(err)
		}
		return
	}
	
	common.ResponseSuccess(ctx, userInfo)
}

// UserInfoHandler 用户主页信息接口
//
//	@Summary		获取用户主页信息的接口
//	@Description	根据 user_id 获取用户信息的接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			user_id	path	int	false	"用户 id"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=models.UserDTO}
//	@Router			/user/{user_id} [get]
func UserHomeHandler(ctx *gin.Context)  {
	userID, err := strconv.ParseInt(ctx.Param("user_id"), 10, 64)
	if err != nil {
		common.ResponseError(ctx, common.CodeInvalidParam)
		return
	}
	userInfo, err := logic.UserGetInfo(userID)
	if err != nil {
		if errors.Is(err, bluebell.ErrUserNotExist) {
			common.ResponseError(ctx, common.CodeUserNotExist)
		} else {
			logger.ErrorWithStack(err)
		}
		return
	}
	
	common.ResponseSuccess(ctx, userInfo)
}

// UserGetPostListHandler 获取用户发布帖子列表接口
//
//	@Summary		获取用户发布帖子列表接口
//	@Description	根据 user_id 获取获取用户发布帖子列表的接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			user_id	query	int	false	"用户 id"
//	@Param			page	query	int	false	"页号"
//	@Param			size	query	int	false	"页的大小"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=models.PostListDTO}
//	@Router			/user/posts [get]
func UserGetPostListHandler(ctx *gin.Context)  {
	params := models.ParamUserPostList{}
	params.PageNum = 1   // default
	params.PageSize = 20

	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))	
		return
	}

	total, postList, err := logic.GetPostListByAuthorID(params)
	if err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		return
	}
	
	common.ResponseSuccess(ctx, models.PostListDTO{
		Total: total,
		Posts: postList,
	})
}

// UserUpdateHandler 用户信息更新接口
//
//	@Summary		用户信息更新接口
//	@Description	用户信息更新接口
//	@Tags			用户相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string					false	"Bearer 用户令牌"
//	@Param			object			body	models.ParamUserUpdate	false	"用户信息"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/user/update [post]
func UserUpdateHandler(ctx *gin.Context)  {
	userID, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.UserUpdateHandler: get user_id from context failed")
		return
	}

	paramUserUpdate := models.ParamUserUpdate{}
	if err := ctx.ShouldBindJSON(&paramUserUpdate); err != nil {
		logger.Debugf("%v", err.Error())
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}

	if err := logic.UserUpdate(userID.(int64), paramUserUpdate); err != nil {
		if errors.Is(err, bluebell.ErrUserExist) {
			common.ResponseErrorWithMsg(ctx, common.CodeUserExist, "用户名已存在")
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, nil)
}

// login: gen aToken and rToken, refresh redis token([key, val]: [user_id, aToken])

// get resource: parse aToken, judge valid

// if valid:
// judge aToken :
// 1. aToken = rdb.aToken, ok
// 2. aToken != rdb.aToken, not ok, relogin

// if not valid:
// return, front end will ask for refresh access token
