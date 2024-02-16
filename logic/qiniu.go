package logic

import (
	"bluebell/dao/qiniu"
	"bluebell/models"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
)

func QiniuGenUploadToken() string {
	accessKey := qiniu.GetAccessKey()
	secretKey := qiniu.GetSecretKey()

	putPolicy := storage.PutPolicy{
		Scope:        qiniu.GetScope(),
		CallbackURL:  qiniu.GetCallbackURL() + "api/v1/qiniu/upload/callback",
		CallbackBody: `{"bucket":"$(bucket)","key":"$(key)","fname":"$(fname)"}`,
		Expires:      qiniu.GetExpires(),
	}

	mac := auth.New(accessKey, secretKey)

	return putPolicy.UploadToken(mac)
}

func QiniuUploadCallback(params models.QiniuCallbackBody) (string, string) {
	baseURL := qiniu.GetBaseURL()
	URL := baseURL + params.Key
	return params.FileName, URL
}
