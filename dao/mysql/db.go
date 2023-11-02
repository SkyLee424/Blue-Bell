package mysql

import (
	"fmt"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

func InitMySQL() {
	dbHost := viper.Get("mysql.host")
	dbPort := viper.GetInt("mysql.port")
	userName := viper.Get("mysql.username")
	password := viper.Get("mysql.password")
	database := viper.Get("mysql.database")
	charset := viper.Get("mysql.charset")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local", userName, password, dbHost, dbPort, database, charset)
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("mysql: %s", err.Error()))
	}

}

func GetDB() *gorm.DB {
	return db
}
