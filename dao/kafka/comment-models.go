package kafka

type CommentCreate struct {
	ObjID     int64  `json:"obj_id,string"`
	ObjType   int8   `json:"obj_type"`
	Root      int64  `json:"root,string"`
	Parent    int64  `json:"parent,string"`
	UserID    int64  `json:"user_id,string"`
	CommentID int64  `json:"comment_id,string"`
	Message   string `json:"message"`
}

type CommentRemove struct {
	ObjID      int64    `json:"obj_id,string"`
	ObjType    int8     `json:"obj_type"`
	CommentID  int64    `json:"comment_id,string"`
	CommentIDs []string `json:"comment_ids"`
	UserID     int64    `json:"user_id,string"`
	IsRoot     bool     `json:"is_root"`
}
