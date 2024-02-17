package models

type User struct {
	ID        int64     `gorm:"type:bigint;auto_increment" json:"id,string"`
	UserID    int64     `gorm:"type:bigint;not null;unique" json:"user_id"`
	UserName  string    `gorm:"type:varchar(64);not null;unique" json:"username"`
	Password  string    `gorm:"type:varchar(64);not null" json:"password"`
	Email     string    `gorm:"type:varchar(64);not null;unique" json:"email"`
	Gender    int8      `gorm:"type:tinyint;default 2;not null" json:"gender"`
	Avatar    string 	`gorm:"type:varchar(256)" json:"avatar"`
	Intro     string 	`gorm:"type:varchar(128)" json:"intro"`
	CreatedAt  Time      `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  Time      `gorm:"type:timestamp default CURRENT_TIMESTAMP" json:"update_at"`
}

type UserDTO struct {
	UserID   int64      `json:"user_id,string"`
	UserName string     `json:"username"`
	Email    string     `json:"email"`
	Gender   int8       `json:"gender"`
	Avatar   string     `json:"avatar"`
	Intro    string     `json:"intro"`
}
