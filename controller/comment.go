package controller

import (
	common "bluebell/controller/Common"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"
	"errors"

	"github.com/gin-gonic/gin"
)

// CreatePostHandler 创建（发送）评论接口
//
//	@Summary		创建（发送）评论接口
//	@Description	创建（发送）评论接口
//	@Tags			评论相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string						false	"Bearer 用户令牌"
//	@Param			object			body	models.ParamCommentCreate	false	"帖子的详细信息"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=models.CommentDTO}
//	@Router			/comment/create [post]
func CommentCreateHandler(ctx *gin.Context) {
	comment := new(models.ParamCommentCreate)
	if err := ctx.ShouldBindJSON(&comment); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}

	userID := ctx.GetInt64("user_id")
	commentDTO, err := logic.CreateComment(comment, userID)
	if err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	common.ResponseSuccess(ctx, commentDTO)

}

// CommentListHandler 评论列表接口
//
//	@Summary		评论列表接口
//	@Description	可以根据楼层（floor）或者点踩数（like）排序的评论列表接口
//	@Tags			评论相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamCommentList	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.CommentListDTO}
//	@Router			/comment/list [get]
func CommentListHandler(ctx *gin.Context) {
	param := &models.ParamCommentList{
		OrderBy:  "floor",
		PageNum:  1,
		PageSize: 10,
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}

	list, err := logic.GetCommentList(param)
	if err != nil {
		if errors.Is(err, bluebell.ErrInvalidParam) {
			common.ResponseError(ctx, common.CodeInvalidParam)
		} else if errors.Is(err, bluebell.ErrNotFound) {
			common.ResponseErrorWithMsg(ctx, common.CodeNotFound, "不存在的主题")
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, list)
}

// CommentRemoveHandler 删除评论接口
//
//	@Summary		删除评论接口
//	@Description	根据 comment_id 删除评论，及子评论
//	@Tags			评论相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string						false	"Bearer 用户令牌"
//	@Param			object			query	models.ParamCommentRemove	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/comment/remove [delete]
func CommentRemoveHandler(ctx *gin.Context) {
	params := &models.ParamCommentRemove{}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}
	userID := ctx.GetInt64("user_id")

	if err := logic.RemoveComment(params, userID); err != nil {
		if errors.Is(err, bluebell.ErrForbidden) {
			common.ResponseError(ctx, common.CodeForbidden)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, nil)
}

// CommentRemoveHandler 评论点赞接口
//
//	@Summary		评论点赞接口
//	@Description	给评论点赞的接口
//	@Tags			评论相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string					false	"Bearer 用户令牌"
//	@Param			object			query	models.ParamCommentLike	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/comment/like [post]
func CommentLikeHandler(ctx *gin.Context) {
	params := &models.ParamCommentLike{}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}
	commentLikeHateHelper(ctx, params.CommentID, params.ObjID, params.ObjType, true)
}

// CommentRemoveHandler 评论点踩接口
//
//	@Summary		评论点踩接口
//	@Description	给评论点踩的接口
//	@Tags			评论相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string					false	"Bearer 用户令牌"
//	@Param			object			query	models.ParamCommentHate	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/comment/hate [post]
func CommentHateHandler(ctx *gin.Context) {
	params := &models.ParamCommentHate{}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}
	commentLikeHateHelper(ctx, params.CommentID, params.ObjID, params.ObjType, false)
}

func commentLikeHateHelper(ctx *gin.Context, commentID, objID int64, objType int8, like bool) {
	userID := ctx.GetInt64("user_id")
	if err := logic.LikeOrHateForComment(userID, commentID, objID, objType, like); err != nil {
		// if errors.Is(err, bluebell.ErrInvalidParam) {
		// 	common.ResponseError(ctx, common.CodeInvalidParam)
		// } else {
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		// }
		return
	}
	common.ResponseSuccess(ctx, nil)
}
