package router

import (
	"bluebell/controller"
	docs "bluebell/docs"
	"bluebell/logger"
	"bluebell/middleware"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var router *gin.Engine

func Init() {
	if !viper.GetBool("server.develop_mode") {
		gin.SetMode(gin.ReleaseMode)
	}

	router = gin.New()
	frontendPath := viper.GetString("router.corf.frontend_path")
	middlewares := []gin.HandlerFunc{logger.GinLogger(), logger.GinRecovery(true), middleware.CORF(frontendPath)}
	if viper.GetBool("router.ratelimit.enable") { // 全局限流
		rate := viper.GetFloat64("router.ratelimit.rate")
		capacity := viper.GetInt64("router.ratelimit.capacity")
		middlewares = append(middlewares, middleware.RateLimit(rate, capacity))
	}
	router.Use(middlewares...)

	/* Swagger 接口文档 */
	if viper.GetBool("service.swagger.enable") {
		docs.SwaggerInfo.BasePath = "/api/v1"
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	v1 := router.Group("/api/v1")

	/* RefreshToken */
	v1.GET("/token/refresh/", controller.RefreshTokenHandler)

	/* User */
	usrGrp := v1.Group("/user")
	usrGrp.POST("/register", controller.UserRegisterHandler)
	usrGrp.POST("/login", controller.UserLoginHandler)
	usrGrp.POST("/update", middleware.Auth(), middleware.VerifyToken(), controller.UserUpdateHandler)
	usrGrp.GET("/info", middleware.Auth(), middleware.VerifyToken(), controller.UserInfoHandler)
	usrGrp.GET("/:user_id", controller.UserHomeHandler)
	usrGrp.GET("/posts", controller.UserGetPostListHandler)

	/* Community */
	communityGrp := v1.Group("/community")
	communityGrp.Use(middleware.Auth(), middleware.VerifyToken())
	communityGrp.POST("/create", controller.CommunityCreateHandler)
	communityGrp.GET("/list", controller.CommunityListHandler)
	communityGrp.GET("/detail", controller.CommunityDetailHandler)

	/* Post */
	postGrp := v1.Group("/post")
	postGrp.Use(middleware.Auth(), middleware.VerifyToken())
	postGrp.POST("/create", controller.CreatePostHandler)
	postGrp.DELETE("/remove", controller.PostRemoveHandler)
	postGrp.GET("/:post_id", controller.PostDetailHandler)
	postGrp.POST("/vote", controller.PostVoteHandler)

	v1.GET("/post/list", controller.PostListHandler)       // 查看列表
	v1.GET("/post/hot", controller.PostHotController)
	if viper.GetBool("elasticsearch.enable") {
		v1.GET("/post/search2", controller.PostSearchHandler2) // 使用 es 实现的搜索
	}
	if viper.GetBool("bleve.enable") {
		v1.GET("/post/search", controller.PostSearchHandler)   // 使用 bleve 实现的搜索
	}

	/* Comment */
	commentGrp := v1.Group("/comment")
	commentGrp.Use(middleware.Auth(), middleware.VerifyToken())
	commentGrp.POST("/create", controller.CommentCreateHandler)
	commentGrp.DELETE("/remove", controller.CommentRemoveHandler)
	commentGrp.POST("/like", controller.CommentLikeHandler)
	commentGrp.POST("/hate", controller.CommentHateHandler)
	commentGrp.GET("/likeOrHateList", controller.CommentUserLikeOrHateListHandler)
	
	v1.GET("/comment/list", controller.CommentListHandler)
	
	/* Qiniu */
	qiniuGrp := v1.Group("/qiniu")
	qiniuGrp.Use(middleware.Auth(), middleware.VerifyToken())
	qiniuGrp.GET("/upload/gentoken", controller.QiniuGenUploadTokenHandler)
	v1.POST("/qiniu/upload/callback", controller.QiniuUploadCallbackHandler) // 这里不需要鉴权

	/* Email */
	emailGrp := v1.Group("/email")
	emailGrp.POST("/verification", controller.EmailSendVerificationCodeHandler)

	/* Empty */
	v1.GET("/empty0", controller.EmptyHandler0) // 空接口
	v1.GET("/empty1", middleware.Auth(), middleware.VerifyToken(), controller.EmptyHandler1) // 空接口，但有鉴权
}

func GetServer() *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", viper.GetString("server.ip"), viper.GetInt("server.port")),
		Handler: router,
	}
}
