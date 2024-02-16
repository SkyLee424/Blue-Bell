package controller

import "bluebell/models"

type ResponseTokens struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type ResponseUserLogin struct {
	UserName     string `json:"user_name"`
	UserID       int64  `json:"user_id,string"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type ResponsePostDetail struct {
	AuthorInfo struct {
		AuthorID   int64  `json:"author_id,string"`
		AuthorName string `json:"author_name"`
	} `json:"author_info"`
	CommunityInfo struct {
		CommunityID   int64       `json:"community_id"`
		CommunityName string      `json:"community_name"`
		Intro         string      `json:"intro"`
		CreatedAt     models.Time `json:"created_at"`
	} `json:"community_info"`
	PostInfo struct {
		PostID    int64       `json:"post_id,string"`
		Title     string      `json:"title"`
		Content   string      `json:"content"`
		CreatedAt models.Time `json:"created_at"`
		UpdatedAt models.Time `json:"updated_at"`
		VoteNum   int64       `json:"vote_num"`
	} `json:"post_info"`
}

type ResponseQiniuCallback struct {
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}
