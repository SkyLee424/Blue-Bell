package kafka

import (
	"bluebell/dao/email"
	"bluebell/dao/redis"
	"fmt"

	"github.com/pkg/errors"
)

func GetEmailSendVerificationCodeUniqueKey(to, code string) string {
	return fmt.Sprintf("email_send_verification_code_%v_%v", to, code)
}

func sendEmailVerificationCode(params EmailSendVerificationCode) (res Result) {
	m := email.GetMailBody(params.To, params.Code)
	if err := email.SendEmail(m); err != nil {
		res.Err = errors.Wrap(err, "kafka:sendEmail: SendEmail")
		return
	}

	// 存到 redis
	// key: user_email
	// value: code
	if err := redis.SetEmailVerificationCode(params.To, params.Code, params.ExpireDuration); err != nil {
		res.Err = errors.Wrap(err, "kafka:sendEmail: SetEmailVerificationCode")
	}
	return
}
