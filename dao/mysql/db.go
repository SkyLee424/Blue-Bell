package mysql

import (
	"bluebell/models"
	"fmt"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	debug := viper.GetBool("mysql.debug")
	var err error
	if debug {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	} else {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}
	if err != nil {
		panic(fmt.Sprintf("mysql: %s", err.Error()))
	}
	initTables()
	initIndices()
}

func initTables() {
	db.AutoMigrate(&models.User{})
	db.AutoMigrate(&models.Community{})
	db.AutoMigrate(&models.Post{})
	db.AutoMigrate(&models.ExpiredPostScore{})
	db.AutoMigrate(&models.CommentSubject{})
	db.AutoMigrate(&models.CommentIndex{})
	db.AutoMigrate(&models.CommentContent{})
	db.AutoMigrate(&models.CommentUserLikeMapping{})
	db.AutoMigrate(&models.CommentUserHateMapping{})
}

func initIndices()  {
	createUnionIndexIfNotExists("idx_cid_uid", "comment_user_like_mappings", "comment_id, user_id")
	createUnionIndexIfNotExists("idx_cid_uid", "comment_user_hate_mappings", "comment_id, user_id")
	createUnionIndexIfNotExists("idx_uid_oid_otype", "comment_user_like_mappings", "user_id, obj_id, obj_type")
	createUnionIndexIfNotExists("idx_uid_oid_otype", "comment_user_like_mappings", "user_id, obj_id, obj_type")
}

func createUnionIndexIfNotExists(indexName, tableName, columns string) {
    var indexCount int64
    db.Raw("SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_NAME = ? AND INDEX_NAME = ?", tableName, indexName).Count(&indexCount)

    if indexCount == 0 {
        // 不存在则创建索引
        db.Exec(fmt.Sprintf("CREATE INDEX %s ON %s(%s);", indexName, tableName, columns))
    }
}

func GetDB() *gorm.DB {
	return db
}
