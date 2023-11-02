package models

type Community struct {
	ID            int64  `gorm:"type:bigint;auto_increment" json:"id"`
	CommunityID   int64  `gorm:"type:bigint;not null;unique" json:"community_id"`
	CommunityName string `gorm:"type:varchar(64);not null;unique" json:"community_name" binding:"required"`
	Introduction  string `gorm:"type:varchar(256);not null" json:"introduction"`
	CreatedAt     Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     Time   `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type CommunityDTO struct {
	CommunityID   int64  `json:"community_id"`
	CommunityName string `json:"community_name" binding:"required"`
	Introduction  string `json:"introduction,omitempty"` // 字段为空则不参与 json 序列化
}
