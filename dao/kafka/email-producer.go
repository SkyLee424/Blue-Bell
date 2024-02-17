package kafka

import (
	"bluebell/dao/email"

	"github.com/pkg/errors"
)

func SendEmailVerificationCode(to, code string) error {
	err := writeMessage(emailWriter, TopicEmail, to, TypeEmailSendVerificationCode, EmailSendVerificationCode{
		To:             to,
		Code:           code,
		ExpireDuration: email.ExpireDuration,
	})

	return errors.Wrap(err, "kafka-producer:SendEmail: writeMessage")
}
