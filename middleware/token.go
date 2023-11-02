package middleware

import (
	"bluebell/logger"
	"bluebell/logic"
	common "bluebell/controller/Common"

	"github.com/gin-gonic/gin"
)

// 校验上下文的 access_token 是否与 redis 中的一致
//
// 如果一致返回 true，否则给客户端发送错误响应，并返回 false
// 感觉直接放在 Auth 中间件，不太合适（依赖 logic 层）
//
// 但是每次都 VerifyToken 感觉很冗余？
func VerifyToken() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := ctx.GetInt64("user_id")
		access_token := ctx.GetString("access_token")

		r_access_token, err := logic.GetUserAccessToken(userID)
		if err != nil {
			logger.ErrorWithStack(err)
			common.ResponseError(ctx, common.CodeInternalErr)
			ctx.Abort()
		} else if access_token != r_access_token {
			common.ResponseError(ctx, common.CodeNeedLogin)
			ctx.Abort()
		}
		ctx.Next()
	}
}
