package email

import (
	"strings"

	"gopkg.in/gomail.v2"
)

func GetMailBody(to, code string) *gomail.Message {
	body := strings.Replace(verificationBody, "123456", code, 1)

	m := gomail.NewMessage()
	m.SetHeader("From", username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Verification Code")
	m.SetBody("text/html", body)

	return m
}
