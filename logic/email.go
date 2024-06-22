package logic

import (
	"bluebell/dao/email"
	"bluebell/dao/kafka"
	"bluebell/dao/redis"
	"bluebell/logger"
	"bluebell/models"
	"crypto/rand"
	"math/big"

	"github.com/pkg/errors"
)

func SendEmailVerificationCode(params models.ParamSendEmailVerificationCode) error {
	code, err := genVerificationCode(email.CodeLen)
	if err != nil {
		return errors.Wrap(err, "logic:SendEmailVerificationCode: genVerificationCode")
	}

	go func() {
		if err := kafka.SendEmailVerificationCode(params.Email, code); err != nil {
			logger.Errorf("logic:SendEmailVerificationCode: send message to kafka failed, reason: %v", err.Error())
		}
	}()
	
	return nil
}

func GetEmailVerificationCode(email string) (string, error) {
	code, err := redis.GetEmailVerificationCode(email)
	return code, errors.Wrap(err, "logic:GetEmailVerificationCode")
}

func genVerificationCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", errors.Wrap(err, "logic:genVerificationCode") // 发生错误，返回错误信息
		}
		code += num.String() // 将数字添加到验证码字符串
	}
	return code, nil
}
