package mysql

import (
	"bluebell/models"

	"github.com/pkg/errors"
)

func SelectUserByUserID(userID int64) (usr *models.User, err error) {
	res := db.First(&usr, "user_id = ?", userID)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "query failed")
	}
	return usr, nil
}

func SelectUserByName(name string) (usr *models.User, err error) {
	res := db.First(&usr, "user_name = ?", name)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "query failed")
	}
	return usr, nil
}

func SelectUserByEmail(email string) (usr *models.User, err error) {
	res := db.First(&usr, "email = ?", email)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "query failed")
	}
	return usr, nil
}

func InsertUser(usr *models.User) error {
	res := db.Create(&usr)
	if res.Error != nil {
		return errors.Wrap(res.Error, "insert failed")
	}
	return nil
}

func UpdateUserInfo(userID int64, params models.ParamUserUpdate) error {
	user := models.User{
		UserName: params.Username,
		Gender: params.Gender,
		Avatar: params.Avatar,
		Intro: params.Intro,
	}

	res := db.Model(&models.User{}).Where("user_id = ?", userID).Updates(user)
	return errors.Wrap(res.Error, "mysql: UpdateUserInfo")
}