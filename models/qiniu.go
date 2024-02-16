package models

type QiniuCallbackBody struct {
	Bucket   string `json:"bucket" binding:"required"`
	Key      string `json:"key" binding:"required"`
	FileName string `json:"fname" binding:"required"`
}
