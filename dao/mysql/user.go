package mysql

import (
	"bluebell/models"

	"github.com/pkg/errors"
)

func SelectUserByID(id int64) (usr *models.User, err error) {
	res := db.First(&usr, "id = ?", id)
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

func InsertUser(usr *models.User) error {
	res := db.Create(&usr)
	if res.Error != nil {
		return errors.Wrap(res.Error, "insert failed")
	}
	return nil
}