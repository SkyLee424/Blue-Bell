package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code `json:"code"` // 业务内部指定的响应码
	Msg  any           `json:"msg"`            // 响应消息
	Data any           `json:"data,omitempty"` // 响应数据
}

func ResponseSuccess(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, &Response{
		Code: CodeSuccess,
		Msg: "成功",
		Data: data,
	})
}

func ResponseError(ctx *gin.Context, code Code) {
	ctx.JSON(http.StatusOK, &Response{
		Code: code,
		Msg: code.getMsg(),
		Data: nil,
	})
}

func ResponseErrorWithMsg(ctx *gin.Context, code Code, msg any) {
	ctx.JSON(http.StatusOK, &Response{
		Code: code,
		Msg: msg,
		Data: nil,
	})
}
