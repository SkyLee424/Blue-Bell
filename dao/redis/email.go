package redis

import (
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

func SetEmailVerificationCode(email_addr, code string, expireDuration time.Duration) error {
	return errors.Wrap(
		set(KeyEmailVerificationCodeStringPF+email_addr, code, expireDuration),
		"redis:SetEmailVerificationCode")
}

func GetEmailVerificationCode(email_addr string) (string, error) {
	cmd := get(KeyEmailVerificationCodeStringPF + email_addr)
	if err := cmd.Err(); err != nil {
		if errors.Is(err, redis.Nil) { // 验证码过期，返回空字符串
			return "", nil
		}
		return "", errors.Wrap(err, "redis:GetEmailVerificationCode")
	}

	return cmd.Val(), nil
}
