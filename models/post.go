package models

type Post struct {
	ID          int64  `gorm:"type:bigint;auto_increment" json:"id"`
	PostID      int64  `gorm:"type:bigint;not null;unique" json:"post_id"`
	CommunityID int64  `gorm:"type:bigint;not null;" json:"community_id" binding:"required"`
	AuthorID    int64  `gorm:"type:bigint;not null;" json:"author_id"`
	Status      int8   `gorm:"type:tinyint;not null;default 1;" json:"status"`
	Title       string `gorm:"type:varchar(128);not null;index:idx_title,class:FULLTEXT" json:"title" binding:"required"`
	Content     string `gorm:"type:varchar(8192);not null;index:idx_content,class:FULLTEXT,size:256" json:"content" binding:"required"`
	CreatedAt   Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type PostDTO struct {
	UserID      int64 `json:"author_id"`
	CommunityID int64 `json:"community_id"`
	PostID      int64 `json:"post_id"`

	CommunityName string `json:"community_name" binding:"required"`
	UserName      string `json:"author_name"`

	CommunityIntro string `json:"community_intro"`
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content" binding:"required"`

	Status             int8 `json:"status"`
	CreatedAt          Time `json:"created_at"`
	UpdatedAt          Time `json:"update_at"`
	CommunityCreatedAt Time `json:"community_created_at"`

	VoteNum int64 `json:"vote_num"`
}
