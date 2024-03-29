package controller

type Code uint

const (
	CodeSuccess Code = iota + 1000
	CodeInternalErr
	CodeServerBusy
	CodeInvalidParam
	CodeNotFound
	CodeUnsupportedAuthProtocol
	CodeInvalidToken
	CodeExpiredToken

	CodeUserExist
	CodeUserNotExist
	CodeWrongPassword
	CodeNeedLogin
	CodeExpiredLogin

	CodeNoSuchCommunity

	CodeNoSuchPost
	CodeVoteTimeExpire

	CodeNoSuchComment

	CodeForbidden
	CodeTimeOut

	CodeInvalidVerificationCode
)

var codeMsgMap = map[Code]string{
	CodeSuccess:                 "成功",
	CodeInternalErr:             "服务繁忙",
	CodeServerBusy:              "触发限流",
	CodeInvalidParam:            "无效参数",
	CodeNotFound:                "未找到",
	CodeUnsupportedAuthProtocol: "不支持的认证协议",
	CodeInvalidToken:            "无效 Token",
	CodeExpiredToken:            "过期 Token",

	CodeUserExist:     "用户已存在",
	CodeUserNotExist:  "用户不存在",
	CodeWrongPassword: "密码错误",
	CodeNeedLogin:     "需要登录",
	CodeExpiredLogin:  "登录过期",

	CodeNoSuchCommunity: "没有该社区",

	CodeNoSuchPost:     "没有该帖子",
	CodeVoteTimeExpire: "超过投票时间",

	CodeNoSuchComment: "没有该评论",

	CodeForbidden: "禁止访问",
	CodeTimeOut: "请求超时",

	CodeInvalidVerificationCode: "无效验证码",
}

func (c Code) getMsg() string {
	msg, ok := codeMsgMap[c]
	if !ok {
		return "无效错误码"
	}
	return msg
}
