package kafka

type LikeOrHateIncr struct {
	Field     string `json:"field"`
	CommentID int64  `json:"comment_id,string"`
	Offset    int    `json:"offset"`
}

type LikeOrHateMappingCreate struct {
	ObjID     int64 `json:"obj_id,string"`
	ObjType   int8  `json:"obj_type"`
	CommentID int64 `json:"comment_id,string"`
	UserID    int64 `json:"user_id,string"`
	Like      bool
}

type LikeOrHateMappingRemove struct {
	CommentID int64 `json:"comment_id,string"`
}
