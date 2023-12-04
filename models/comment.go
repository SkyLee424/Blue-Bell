package models

type CommentSubject struct {
	ID        int64 `gorm:"primaryKey" json:"id"`
	ObjID     int64 `gorm:"column:obj_id" json:"obj_id"`
	ObjType   int8  `gorm:"column:obj_type" json:"obj_type"`
	Count     int   `gorm:"column:count" json:"count"`
	RootCount int   `gorm:"column:root_count" json:"root_count"`
	Status    int8  `gorm:"column:status;default:0" json:"status"`
	CreatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type CommentIndex struct {
	ID            int64 `gorm:"primaryKey" json:"id"`
	ObjID         int64 `gorm:"column:obj_id" json:"obj_id"`
	ObjType       int8  `gorm:"column:obj_type" json:"obj_type"`
	Root          int64 `gorm:"column:root" json:"root"`
	Parent        int64 `gorm:"column:parent" json:"parent"`
	UserID        int64 `gorm:"column:user_id" json:"user_id"`
	Floor         int   `gorm:"column:floor" json:"floor"`
	Count         int   `gorm:"column:count;default:0" json:"count"`
	RootCount     int   `gorm:"column:root_count" json:"root_count"`
	Like          int   `gorm:"column:like;default:0" json:"like"`
	Hate          int   `gorm:"column:hate;default:0" json:"hate"`
	Status        int8  `gorm:"column:status;default:0" json:"status"`
	AuthorLiked   bool  `gorm:"column:author_liked;default:false" json:"author_liked"`
	AuthorReplied bool  `gorm:"column:author_replied;default:false" json:"author_replied"`
	CreatedAt     Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type CommentContent struct {
	CommentID int64  `gorm:"primaryKey" json:"comment_id"`
	Message   string `gorm:"type:varchar(8192);comment:评论内容" json:"message"`
	CreatedAt Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

// 记录一个 comment 有哪些用户点过赞
type CommentUserLikeMapping struct {
	ID        int64 `gorm:"primaryKey" json:"id"`
	CommentID int64 `json:"comment_id"`
	UserID    int64 `json:"user_id"`
	ObjID     int64 `json:"obj_id"`
	ObjType   int8  `json:"obj_type"`
	CreatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type CommentUserHateMapping struct {
	ID        int64 `gorm:"primaryKey" json:"id"`
	CommentID int64 `json:"comment_id"`
	UserID    int64 `json:"user_id"`
	ObjID     int64 `json:"obj_id"`
	ObjType   int8  `json:"obj_type"`
	CreatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt Time  `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type CommentDTO struct {
	CommentID int64 `json:"comment_id"`
	ObjID     int64 `json:"obj_id"`
	Type      int8  `json:"type"`
	Root      int64 `json:"root"`
	Parent    int64 `json:"parent"`
	UserID    int64 `json:"user_id"`
	Floor     int   `json:"floor"`
	Like      int   `json:"like"`
	Content   struct {
		Message string `json:"message"`
	} `json:"content"`
	Replies      []CommentDTO `json:"replies"`
	AuthorAction struct {
		Liked   bool `json:"liked"`
		Replied bool `json:"replied"`
	} `json:"author_action"`
	CreatedAt Time `json:"created_at"`
	UpdatedAt Time `json:"update_at"`
}

type CommentListDTO struct {
	Total    int          `json:"total"`
	Comments []CommentDTO `json:"comments"`
}

type CommentContentDTO struct {
	Message string `json:"message"`
}
