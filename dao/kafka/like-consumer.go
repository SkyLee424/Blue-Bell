package kafka

import (
	"bluebell/dao/mysql"
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func GetIncrCommentIndexCountFieldUniqueKey(field string, commentID int64) string {
	return fmt.Sprintf("incr_comment_index_%v_%v", field, commentID)
}

func GetCreateCommentLikeOrHateUserUniqueKey(like bool, commentID int64) string {
	return fmt.Sprintf("create_comment_mapping_%v_%v", like, commentID)
}

func GetRemoveCommentUserLikeMappingByCommentIDsUniqueKey(commentID int64) string {
	return fmt.Sprintf("remove_comment_mapping_like_%v", commentID)
}

func incrCommentIndexCountField(tx *gorm.DB, field string, commentID int64, offset int) (res Result) {
	res.UniqueKey = GetIncrCommentIndexCountFieldUniqueKey(field, commentID)

	if err := mysql.IncrCommentIndexCountField(tx, field, commentID, offset); err != nil {
		res.Err = errors.Wrap(err, "kafka:incrCommentIndexCountField")
	}

	return
}

func createCommentLikeOrHateUser(tx *gorm.DB, commentID, userID, objID int64, objType int8, like bool) (res Result) {
	res.UniqueKey = GetCreateCommentLikeOrHateUserUniqueKey(like, commentID)

	if err := mysql.CreateCommentLikeOrHateUser(tx, commentID, userID, objID, objType, like); err != nil {
		res.Err = errors.Wrap(err, "kafka:createCommentLikeOrHateUser")
	}

	return
}

func removeCommentUserLikeMappingByCommentIDs(tx *gorm.DB, commentID int64) (res Result) {
	res.UniqueKey = GetRemoveCommentUserLikeMappingByCommentIDsUniqueKey(commentID)

	if err := mysql.DeleteCommentUserLikeMappingByCommentIDs(tx, []int64{commentID}); err != nil {
		res.Err = errors.Wrap(err, "kafka:removeCommentUserLikeMappingByCommentIDs")
	}

	return
}
