package models

type ExpiredPostScore struct {
	PostID      int64 `json:"post_id"`
	PostScore   int64 `json:"post_score"`
	PostVoteNum int64 `json:"post_vote_num"`
}
