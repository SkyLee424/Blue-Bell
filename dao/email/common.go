package email

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/gomail.v2"
)

var (
	username         string
	verificationBody string
	mailer           *gomail.Dialer

	CodeLen        int
	ExpireDuration time.Duration
)

func InitEmail() {
	username = viper.GetString("email.username")
	password := viper.GetString("email.password")
	host := viper.GetString("email.host")
	port := viper.GetInt("email.port")

	mailer = gomail.NewDialer(host, port, username, password)

	bodyPath := viper.GetString("email.verification.body_path")
	content, err := os.ReadFile(bodyPath)
	if err != nil {
		panic(err)
	}
	verificationBody = string(content)

	CodeLen = viper.GetInt("email.verification.length")
	ExpireDuration = time.Second * time.Duration(viper.GetInt("email.verification.expire_time"))
}

func SendEmail(m ...*gomail.Message) error {
	return errors.Wrap(mailer.DialAndSend(m...), "email:SendVerificationCode")
}
