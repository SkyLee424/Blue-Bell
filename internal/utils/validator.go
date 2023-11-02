package utils

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/spf13/viper"
)

var trans ut.Translator

// InitTrans 初始化翻译器
func InitTrans() {
	lang := viper.GetString("server.lang") // 读取配置文件
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {

		// 利用反射获取 Tag
		v.RegisterTagNameFunc(func(field reflect.StructField) string {
			return field.Tag.Get("json")
		})

		zhT := zh.New() // 中文翻译器
		enT := en.New() // 英文翻译器

		uni := ut.New(enT, zhT, enT)

		// lang 通常取决于 http 请求头的 'Accept-Language'
		var ok bool
		trans, ok = uni.GetTranslator(lang)
		if !ok {
			panic(fmt.Errorf("uni.GetTranslator(%s) failed", lang))
		}

		var err error
		switch lang {
		case "en":
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		case "zh":
			err = zhTranslations.RegisterDefaultTranslations(v, trans)
		default:
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		}
		if err != nil {
			panic(err.Error())
		}
	}
}

// 解析 err
func ParseToValidationError(err error) any {
	var res any
	if v, ok := err.(validator.ValidationErrors); ok {
		res = v.Translate(GetTranslator())
	} else {
		res = "无效参数"
	}
	return res
}

func GetTranslator() ut.Translator {
	return trans
}
