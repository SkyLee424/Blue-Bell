package bluebell

import "github.com/pkg/errors"

var (
	// user
	ErrUserExist     = errors.New("用户已经存在")
	ErrUserNotExist  = errors.New("用户不存在")
	ErrWrongPassword = errors.New("密码错误")

	// common
	ErrGenToken     = errors.New("生成 Token 失败")
	ErrInvalidToken = errors.New("无效的 Token")
	ErrExpiredToken = errors.New("过期的 Token")
	ErrInternal     = errors.New("服务器繁忙")

	// community
	ErrNoSuchCommunity = errors.New("没有该社区")

	// post
	ErrNoSuchPost     = errors.New("没有该帖子")
	ErrVoteTimeExpire = errors.New("超过投票时间")
)
