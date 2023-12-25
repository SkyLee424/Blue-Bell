package logic

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	"bluebell/internal/utils"
	"bluebell/models"

	bluebell "bluebell/errors"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func CheckUserIfExist(username string) (bool, error) {
	_, err := mysql.SelectUserByName(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil // 不存在
		}
		return false, errors.Wrap(err, "query error") // 发生其它错误
	}
	return true, nil // 存在
}

func Regist(usr *models.User) (string, string, error) {
	// 查询用户是否存在
	exist, err := CheckUserIfExist(usr.UserName)
	if err != nil {
		return "", "", errors.Wrap(err, "check user if exist error")
	}
	if exist {
		return "", "", bluebell.ErrUserExist
	}

	// 不存在，新建用户
	// 加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(usr.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", errors.Wrap(err, "加密错误")
	}
	usr.Password = string(hashedPassword)

	// 创建 user_id
	usr.UserID = utils.GenSnowflakeID()

	// 持久化
	if err := mysql.InsertUser(usr); err != nil {
		return "", "", errors.Wrap(err, "添加用户失败")
	}

	return genTokenHelper(usr.UserID)
}

func Login(usr *models.User) (string, string, error) {
	// 判断用户是否存在
	exist, err := CheckUserIfExist(usr.UserName)
	if err != nil {
		return "", "", errors.Wrap(err, "check user if exist")
	}
	if !exist {
		return "", "", bluebell.ErrUserNotExist
	}

	// 查询、解析密码
	_usr, err := mysql.SelectUserByName(usr.UserName)
	if err != nil {
		return "", "", err
	}

	// 验证密码一致性
	if err := bcrypt.CompareHashAndPassword([]byte(_usr.Password), []byte(usr.Password)); err != nil {
		return "", "", bluebell.ErrWrongPassword
	}

	// 刷新 access_token、refresh_token 并返回
	usr.UserID = _usr.UserID
	return genTokenHelper(_usr.UserID)
}

// 刷新 access_token、refresh_token 并返回
func genTokenHelper(UserID int64) (string, string, error) {
	// 生成 access_token
	access_token, err0 := utils.GenToken(UserID, utils.AccessType)
	refresh_token, err1 := utils.GenToken(0, utils.RefreshType)
	if err0 != nil || err1 != nil {
		return "", "", bluebell.ErrGenToken
	}

	// 刷新 redis 中的 access_token
	// 刷新 redis 中的 refresh_token
	if err := redis.SetUserAccessToken(UserID, access_token, utils.GetAccessTokenExpireDuration()); err != nil {
		return "", "", err
	}
	if err := redis.SetUserRefreshToken(UserID, refresh_token, utils.GetRefreshTokenExpireDuration()); err != nil {
		return "", "", err
	}

	return access_token, refresh_token, nil
}
