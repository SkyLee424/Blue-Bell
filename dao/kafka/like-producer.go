package kafka

import (
	"strconv"

	"github.com/pkg/errors"
)

func IncrCommentIndexCountField(field string, id int64, offset int) error {
	err := writeMessage(likeWriter, TopicLike, strconv.Itoa(int(id)), TypeLikeOrHateIncr, LikeOrHateIncr{
		Field:     field,
		CommentID: id,
		Offset:    offset,
	})

	return errors.Wrap(err, "kafka-producer:IncrCommentIndexCountField: writeMessage")
}

func CreateCommentLikeOrHateUser(commentID, userID, objID int64, objType int8, like bool) error {
	err := writeMessage(likeWriter, TopicLike, strconv.Itoa(int(commentID)), TypeLikeOrHateMappingCreate, LikeOrHateMappingCreate{
		CommentID: commentID,
		UserID:    userID,
		ObjID:     objID,
		ObjType:   objType,
		Like:      like,
	})

	return errors.Wrap(err, "kafka-producer:CreateCommentLikeOrHateUser: writeMessage")
}

func RemoveCommentUserLikeMapping(commentID int64) error {

	err := writeMessage(likeWriter, TopicLike, strconv.Itoa(int(commentID)), TypeLikeOrHateMappingRemove, LikeOrHateMappingRemove{
		CommentID: commentID,
	})

	return errors.Wrap(err, "kafka-producer:RemoveCommentUserLikeMapping: writeMessage")
}
