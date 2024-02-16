package kafka

import (
	"bluebell/models"
	"strconv"

	"github.com/pkg/errors"
)

func CreateComment(params models.ParamCommentCreate, userID, commentID int64) error {
	content := CommentCreate{
		ObjID:     params.ObjID,
		ObjType:   params.ObjType,
		Root:      params.Root,
		Parent:    params.Parent,
		UserID:    userID,
		CommentID: commentID,
		Message:   params.Message,
	}
	err := writeMessage(commentWriter, TopicComment, strconv.FormatInt(commentID, 10), TypeCommentCreate, content)
	return errors.Wrap(err, "kafka-producer:CreateComment: writeMessage")
}

func RemoveComment(params models.ParamCommentRemove, userID int64, commentIDs []int64, isRoot bool) error {
	commentIDStrs := make([]string, len(commentIDs))
	for i := 0; i < len(commentIDs); i++ {
		commentIDStrs[i] = strconv.FormatInt(commentIDs[i], 10)
	}
	content := CommentRemove{
		ObjID:      params.ObjID,
		ObjType:    params.ObjType,
		CommentID:  params.CommentID,
		IsRoot:     isRoot,
		CommentIDs: commentIDStrs,
		UserID:     userID,
	}

	err := writeMessage(commentWriter, TopicComment, strconv.Itoa(int(content.CommentID)), TypeCommentRemove, content)
	return errors.Wrap(err, "kafka-producer:RemoveComment: writeMessage")
}

func RemoveCommentsByObjID(objID int64, objType int8) error {

	content := CommentRemoveByObjID{
		ObjID:   objID,
		ObjType: objType,
	}

	err := writeMessage(commentWriter, TopicComment, strconv.Itoa(int(content.ObjID)), TypeCommentRemoveByObjID, content)
	return errors.Wrap(err, "kafka-producer:RemoveCommentsByObjID: writeMessage")
}
