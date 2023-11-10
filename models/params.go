package models

/*
	存放所有有关请求参数的结构体
*/

/* User */
type ParamUserRegist struct {
	Username   string `json:"username" binding:"required,min=3,max=64"`
	Password   string `json:"password" binding:"required,min=6,max=64"`
	RePassword string `json:"re_password" binding:"required,eqfield=Password"`
}

type ParamUserLogin struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=64"`
}

/* Post */
type ParamCreatePost struct {
	CommunityID int64  `json:"community_id" binding:"required"`
	Title       string `json:"title" binding:"required,min=1,max=128"`
	Content     string `json:"content" binding:"required,max=8192"`
}

type ParamVote struct {
	PostID    int64 `json:"post_id" binding:"required"`
	Direction int8  `json:"direction" binding:"oneof=1 0 -1"`
}

type ParamPostList struct {
	PageNum     int64  `form:"page" binding:"gt=0" example:"1"`    // 页码
	PageSize    int64  `form:"size" binding:"gt=0" example:"10"`   // 每页展示的 post 的数量
	OrderBy     string `form:"orderby" binding:"oneof=time score"` // 排序方式
	CommunityID int64  `form:"community_id" example:"1"`           // 社区 id
}

type ParamPostListByKeyword struct {
	PageNum  int64  `form:"page" binding:"gt=0" example:"1"`          // 页码
	PageSize int64  `form:"size" binding:"gt=0" example:"10"`         // 每页展示的 post 的数量
	OrderBy  string `form:"orderby" binding:"oneof=time correlation"` // 排序方式
	Keyword  string `form:"keyword" binding:"required"`               // 关键字
}
