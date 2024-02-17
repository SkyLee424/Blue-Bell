package kafka

import (
	"time"
)

type EmailSendVerificationCode struct {
	To             string        `json:"to"`
	Code           string        `json:"code"`
	ExpireDuration time.Duration `json:"expire_duration"`
}
