package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 解决跨域问题
func CORF (frontendPath string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", frontendPath)
		ctx.Writer.Header().Set("Access-Control-Allow-Headers", "*")
		ctx.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		ctx.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusOK)
			return
		}
		ctx.Next()
	}
}