package mysql

import "gorm.io/gorm"

func getUseDB(tx *gorm.DB) *gorm.DB{
	if tx != nil {
		return tx
	}
	return db
}