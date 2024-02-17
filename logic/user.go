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

func UserRegist(usr *models.User) (string, string, error) {
	// 查询用户是否存在
	exist, _, err := checkUserIfExist(usr.UserName)
	if err != nil {
		return "", "", errors.Wrap(err, "logic:UserLogin: checkUserIfExist")
	}
	if exist {
		return "", "", bluebell.ErrUserExist
	}

	// 不存在，新建用户
	// 加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(usr.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", errors.Wrap(err, "logic:UserLogin: GenerateFromPassword")
	}
	usr.Password = string(hashedPassword)

	// 创建 user_id
	usr.UserID = utils.GenSnowflakeID()

	// 持久化
	if err := mysql.InsertUser(usr); err != nil {
		return "", "", errors.Wrap(err, "logic:UserLogin: InsertUser")
	}

	return genTokenHelper(usr.UserID)
}

func UserLogin(usr *models.User) (string, string, error) {
	// 判断用户是否存在
	exist, _, err := checkUserIfExist(usr.UserName)
	if err != nil {
		return "", "", errors.Wrap(err, "logic:UserLogin: checkUserIfExist")
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

	usr.UserID = _usr.UserID
	usr.Avatar = _usr.Avatar
	usr.Gender = _usr.Gender
	usr.Email = _usr.Email
	usr.Intro = _usr.Intro

	// 刷新 access_token、refresh_token 并返回
	access_token, refresh_token, err := genTokenHelper(_usr.UserID)
	return access_token, refresh_token, errors.Wrap(err, "logic:UserLogin: genTokenHelper")
}

func UserUpdate(userID int64, params models.ParamUserUpdate) error {
	exist, _userID, err := checkUserIfExist(params.Username)
	if err != nil {
		return errors.Wrap(err, "logic:UserUpdate: CheckUserIfExist")
	}
	if exist && userID != _userID {
		return bluebell.ErrUserExist
	}

	err = mysql.UpdateUserInfo(userID, params)
	return errors.Wrap(err, "logic:UserUpdate: UpdateUserInfo")
}

func UserGetInfo(userID int64) (*models.UserDTO, error) {
	user, err := mysql.SelectUserByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bluebell.ErrUserNotExist
		}
		return nil, errors.Wrap(err, "logic:UserGetInfo: SelectUserByUserID")
	}

	return &models.UserDTO{
		UserID: userID,
		UserName: user.UserName,
		Email: user.Email,
		Gender: user.Gender,
		Avatar: user.Avatar,
		Intro: user.Intro,
	}, nil
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

func checkUserIfExist(username string) (bool, int64, error) {
	usr, err := mysql.SelectUserByName(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, 0, nil // 不存在
		}
		return false, 0, errors.Wrap(err, "logic:checkUserIfExist: SelectUserByName") // 发生其它错误
	}
	return true, usr.UserID, nil // 存在
}