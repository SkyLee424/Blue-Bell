package controller

import (
	common "bluebell/controller/Common"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"
	"bluebell/objects"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	DefaultPageNum  = 1
	DefaultPageSize = 10
	DefaultOrderBy  = "time"
)

// CreatePostHandler 创建帖子接口
//
//	@Summary		创建帖子接口
//	@Description	创建帖子的接口
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string					false	"Bearer 用户令牌"
//	@Param			object			body	models.ParamCreatePost	false	"帖子的详细信息"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/post/create [post]
func CreatePostHandler(ctx *gin.Context) {
	// 解析数据
	post := new(models.Post)
	// 使用 validator 在解析数据的同时做参数校验
	if err := ctx.ShouldBindJSON(&post); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	if _, err := logic.GetCommunityDetailByID(post.CommunityID); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, "不存在的社区")
		return
	}

	// 获取 user_id
	value, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.CreatePostHandler: get user_id from context failed")
		return
	}
	userID, ok := value.(int64)
	if !ok {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.CreatePostHandler: convert user_id from context to int64 failed")
		return
	}

	post.AuthorID = userID

	// 生成 post_id
	post.PostID = utils.GenSnowflakeID()

	// 持久化
	if err := logic.CreatePost(post); err != nil {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.ErrorWithStack(err)
		return
	}

	// 返回
	common.ResponseSuccess(ctx, nil) // 暂时返回 nil
}

// CreatePostHandler 获取帖子详情接口
//
//	@Summary		获取帖子详情接口
//	@Description	创建帖子的接口
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string	false	"Bearer 用户令牌"
//	@Param			post_id			path	int		false	"帖子 id"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=common.ResponsePostDetail}
//	@Router			/post/{post_id} [get]
func PostDetailHandler(ctx *gin.Context) {
	// 解析参数
	value, exists := ctx.Params.Get("post_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInvalidParam)
		return
	}
	post_id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		common.ResponseError(ctx, common.CodeInvalidParam)
		return
	}

	post, err := logic.GetPostDetailByID(post_id, true)
	if err != nil {
		if errors.Is(err, bluebell.ErrNoSuchPost) {
			common.ResponseError(ctx, common.CodeNoSuchPost)
		} else if errors.Is(err, bluebell.ErrTimeout) {
			common.ResponseError(ctx, common.CodeTimeOut)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			// 打日志
			logger.ErrorWithStack(err)
		}
		return
	}

	// 合并一下，方便看
	common.ResponseSuccess(ctx, &common.ResponsePostDetail{
		AuthorInfo: struct {
			AuthorID   int64  "json:\"author_id,string\""
			AuthorName string "json:\"author_name\""
		}{
			AuthorID:   post.UserID,
			AuthorName: post.UserName,
		},
		CommunityInfo: struct {
			CommunityID   int64       "json:\"community_id\""
			CommunityName string      "json:\"community_name\""
			Intro         string      "json:\"intro\""
			CreatedAt     models.Time "json:\"created_at\""
		}{
			CommunityID:   post.CommunityID,
			CommunityName: post.CommunityName,
			Intro:         post.CommunityIntro,
			CreatedAt:     post.CommunityCreatedAt,
		},
		PostInfo: struct {
			PostID    int64       "json:\"post_id,string\""
			Title     string      "json:\"title\""
			Content   string      "json:\"content\""
			CreatedAt models.Time "json:\"created_at\""
			UpdatedAt models.Time "json:\"updated_at\""
			VoteNum   int64       "json:\"vote_num\""
		}{
			PostID:    post.PostID,
			Title:     post.Title,
			Content:   post.Content,
			VoteNum:   post.VoteNum,
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		},
	})
}

// PostVoteHandler 帖子投票接口
//
//	@Summary		帖子投票接口
//	@Description	给指定的帖子投票的接口
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string				false	"Bearer 用户令牌"
//	@Param			object			body	models.ParamVote	false	"投票参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/post/vote [post]
func PostVoteHandler(ctx *gin.Context) {
	// 解析数据
	var params models.ParamVote
	if err := ctx.ShouldBindJSON(&params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	value, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.PostVoteHandler: get user_id from context failed")
		return
	}

	user_id := value.(int64)

	if err := logic.VoteForPost(user_id, params.PostID, params.Direction); err != nil {
		if errors.Is(err, bluebell.ErrNoSuchPost) {
			common.ResponseError(ctx, common.CodeNoSuchPost)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			// 打日志
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, nil)
}

// PostListHandler 帖子列表接口
//
//	@Summary		帖子列表接口
//	@Description	按社区按时间(time)或分数(score)排序查询帖子列表接口
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamPostList	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.PostListDTO}
//	@Router			/post/list [get]
func PostListHandler(ctx *gin.Context) {
	// 解析数据
	params := &models.ParamPostList{
		PageNum:     DefaultPageNum,
		PageSize:    DefaultPageSize,
		OrderBy:     DefaultOrderBy,
		CommunityID: -1,
	}

	if err := ctx.ShouldBindQuery(params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}

	list, total, err := logic.GetAllPostList(params)

	if err != nil {
		if errors.Is(err, bluebell.ErrInvalidParam) {
			common.ResponseError(ctx, common.CodeInvalidParam)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	common.ResponseSuccess(ctx, &models.PostListDTO{
		Total: total,
		Posts: list,
	})
}

// PostSearchHandler 帖子搜索接口
//
//	@Summary		帖子搜索接口
//	@Description	使用 bleve 实现，根据关键字搜索帖子，包含过期帖子
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamPostListByKeyword	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.PostListDTO}
//	@Router			/post/search [get]
func PostSearchHandler(ctx *gin.Context) {
	// 解析数据
	params := &models.ParamPostListByKeyword{
		PageNum:  DefaultPageNum,
		PageSize: DefaultPageSize,
		OrderBy:  "correlation", // 这里使用相关性作为默认排序规则
	}
	if err := ctx.ShouldBindQuery(params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}
	// 拒绝服务
	if params.PageNum*params.PageSize >= 1e4 {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, "Too much data requested")
		return
	}

	// 关键字检索
	postList, total, err := logic.GetPostListByKeyword(params)
	if err != nil {
		if errors.Is(err, bluebell.ErrInvalidParam) {
			common.ResponseError(ctx, common.CodeInvalidParam)
			return
		}
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	// 返回帖子列表
	common.ResponseSuccess(ctx, &models.PostListDTO{
		Total: total,
		Posts: postList,
	})
}

// PostSearchHandler2 帖子搜索接口
//
//	@Summary		帖子搜索接口
//	@Description	使用 elasticsearch 实现，根据关键字搜索帖子，包含过期帖子
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			object	query	models.ParamPostListByKeyword	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.PostListDTO}
//	@Router			/post/search2 [get]
func PostSearchHandler2(ctx *gin.Context) {
	// 解析数据
	params := &models.ParamPostListByKeyword{
		PageNum:  DefaultPageNum,
		PageSize: DefaultPageSize,
		OrderBy:  "correlation", // 这里使用相关性作为默认排序规则
	}
	if err := ctx.ShouldBindQuery(params); err != nil {
		msg := utils.ParseToValidationError(err)
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, msg)
		return
	}
	// 拒绝服务
	if params.PageNum*params.PageSize >= 1e4 {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, "Too much data requested")
		return
	}

	// 关键字检索
	postList, total, err := logic.GetPostListByKeyword2(params)
	if err != nil {
		if errors.Is(err, bluebell.ErrInvalidParam) {
			common.ResponseError(ctx, common.CodeInvalidParam)
			return
		}
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	// 返回帖子列表
	common.ResponseSuccess(ctx, &models.PostListDTO{
		Total: total,
		Posts: postList,
	})
}

// PostHotController 火热帖子列表接口
//
//	@Summary		火热帖子列表接口
//	@Description	获取火热帖子列表
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response{data=[]models.PostListDTO}
//	@Router			/post/hot [get]
func PostHotController(ctx *gin.Context) {
	list, err := logic.GetHotPostList()
	if err != nil {
		if errors.Is(err, bluebell.ErrTimeout) {
			common.ResponseError(ctx, common.CodeTimeOut)	
			return
		}
		common.ResponseError(ctx, common.CodeInternalErr)
		logger.ErrorWithStack(err)
		return
	}

	common.ResponseSuccess(ctx, &models.PostListDTO{
		Total: len(list),
		Posts: list,
	})
}

// PostRemoveHandler 删除帖子接口
//
//	@Summary		删除帖子接口
//	@Description	根据 post_id 删除帖子及其下所有评论
//	@Tags			帖子相关接口
//	@Accept			application/json
//	@Produce		application/json
//	@Param			Authorization	header	string					false	"Bearer 用户令牌"
//	@Param			object			query	models.ParamPostRemove	false	"查询参数"
//	@Security		ApiKeyAuth
//	@Success		200	{object}	common.Response
//	@Router			/post/remove [delete]
func PostRemoveHandler(ctx *gin.Context)  {
	params := models.ParamPostRemove{}

	if err := ctx.ShouldBindQuery(&params); err != nil {
		common.ResponseErrorWithMsg(ctx, common.CodeInvalidParam, utils.ParseToValidationError(err))
		return
	}

	value, exists := ctx.Get("user_id")
	if !exists {
		common.ResponseError(ctx, common.CodeInternalErr)
		// 打日志
		logger.Errorf("controller.PostVoteHandler: get user_id from context failed")
		return
	}
	userID := value.(int64)

	// 删除帖子
	if err := logic.RemovePost(userID, params); err != nil {
		if errors.Is(err, bluebell.ErrForbidden) {
			common.ResponseError(ctx, common.CodeForbidden)
		} else {
			common.ResponseError(ctx, common.CodeInternalErr)
			logger.ErrorWithStack(err)
		}
		return
	}

	// 删除评论
	// 根据 objID、objType 删除其下所有评论
	logic.RemoveCommentsByObjID(params.PostID, objects.ObjPost)
	
	common.ResponseSuccess(ctx, nil)
}