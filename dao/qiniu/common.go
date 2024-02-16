package qiniu

import "github.com/spf13/viper"

var (
	accessKey       string
	secretKey       string
	scope           string
	expires         uint64
	baseURL         string
	callbackBaseURL string
)

func InitQiniuConfig() {
	accessKey = viper.GetString("qiniu.access_key")
	secretKey = viper.GetString("qiniu.secret_key")
	scope = viper.GetString("qiniu.scope")
	expires = viper.GetUint64("qiniu.expires")
	baseURL = viper.GetString("qiniu.base_url")
	callbackBaseURL = viper.GetString("qiniu.callback_base_url")
}

func GetAccessKey() string {
	return accessKey
}

func GetSecretKey() string {
	return secretKey
}

func GetScope() string {
	return scope
}

func GetExpires() uint64 {
	return expires
}

func GetBaseURL() string {
	return baseURL
}

func GetCallbackURL() string {
	return callbackBaseURL
}