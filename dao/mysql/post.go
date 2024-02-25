package mysql

import (
	"bluebell/models"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func CreatePost(post *models.Post) error {
	res := db.Create(&post)
	return errors.Wrap(res.Error, "create post")
}

func CreateExpiredPostScores(tx *gorm.DB, posts []models.ExpiredPostScore) error {
	useDB := getUseDB(tx)
	err := useDB.Transaction(func(tx1 *gorm.DB) error {
		for _, post := range posts {
			res := tx1.Create(&post)
			if res.Error != nil {
				return errors.Wrap(res.Error, "")
			}
		}
		return nil
	})
	return errors.Wrap(err, "Create Expired Post Scores")
}

func SelectPostByID(postID int64) (*models.Post, error) {
	post := new(models.Post)
	res := db.First(post, "post_id = ?", postID)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get post")
	}
	return post, nil
}

func SelectPostDetailByID(postID int64) (*models.PostDTO, error) {
	detail := new(models.PostDTO)
	sqlStr := `SELECT u.user_id,
				c.community_id,
				p.post_id,
				u.user_name,
				c.community_name,
				c.introduction community_intro,
				c.created_at community_created_at,
				p.status,
				p.title,
				p.content,
				p.created_at,
				p.updated_at
			FROM posts p
			JOIN bluebell.communities c ON c.community_id = p.community_id
			JOIN users u ON u.user_id = p.author_id
			WHERE p.post_id = ?`

	res := db.Raw(sqlStr, postID).Scan(detail)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get post detail")
	}
	return detail, nil
}

func SelectPostListByPostIDs(postIDs []string) ([]*models.PostDTO, error) {
	// 限制 content 的长度
	contentLength := viper.GetInt64("service.post.content_max_length")
	sqlStr := `SELECT u.user_id,
				c.community_id,
				p.post_id,
				u.user_name,
				c.community_name,
				p.status,
				p.title,
				substr(p.content, 1, ?) content,
				p.created_at,
				p.updated_at
			FROM posts p
			JOIN bluebell.communities c ON c.community_id = p.community_id
			JOIN users u ON u.user_id = p.author_id
			WHERE p.post_id IN ?
			ORDER BY FIND_IN_SET(p.post_id, ?);`

	list := make([]*models.PostDTO, 0, len(postIDs)) // 提前 准备好容量，避免扩容

	// 将 postIDs 切片连接成逗号分隔的字符串
	postIDsStr := strings.Join(postIDs, ",")
	res := db.Raw(sqlStr, contentLength, postIDs, postIDsStr).Scan(&list)

	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get post list")
	}
	return list, nil
}

// 按照指定 ID 顺序，返回 voteNums
//
// 注意，该方法只能用于查询过期帖子
func SelectPostVoteNumsByIDs(postIDs []string) ([]int64, error) {
	sqlStr := `select post_vote_num
	from expired_post_scores
	where post_id in ?
	order by FIND_IN_SET(post_id, ?);
	`
	postIDsStr := strings.Join(postIDs, ",")
	voteNums := make([]int64, 0, len(postIDs))

	res := db.Raw(sqlStr, postIDs, postIDsStr).Scan(&voteNums) // 走主键索引
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "select post vote num by ids")
	}

	return voteNums, nil
}

func SelectPostsByAuthorID(authorID int64, start, size int) ([]*models.PostDTO, error) {
	postList := make([]*models.PostDTO, 0)
	contentLength := viper.GetInt64("service.post.content_max_length")
	sqlStr := `select post_id, status, title, created_at, updated_at, substr(content, 1, ?) as content
	from posts
	where author_id = ?
	limit ? offset ?`
	// res := db.Model(&models.Post{}).Select("post_id", "title", "created_at", "status").Where("author_id = ?", authorID).Limit(size).Offset(start).Scan(&postList)
	res := db.Raw(sqlStr, contentLength, authorID, size, start).Scan(&postList)
	
	return postList, errors.Wrap(res.Error, "mysql: SelectPostsByAuthorID")
} 

func SelectPostCountByAuthorID(authorID int64) (int, error) {
	total := 0
	res := db.Model(&models.Post{}).Select("count(*)").Where("author_id = ?", authorID).Scan(&total)
	
	return total, errors.Wrap(res.Error, "mysql:SelectPostCountByAuthorID")
}

func UpdatePostStatusByPostIDs(tx *gorm.DB, status int8, postIDs []string) error {
	useDB := getUseDB(tx)
	res := useDB.Model(&models.Post{}).Where("post_id in ?", postIDs).Update("status", status) // 走主键索引

	return errors.Wrap(res.Error, "update post status by post_ids")
}

func DeletePostDetailByPostID(tx *gorm.DB, postID int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.Post{}, "post_id = ?", postID)
	
	return errors.Wrap(res.Error, "mysql:DeletePostDetailByPostID")
}

func DeletePostExpiredScoresByPostID(tx *gorm.DB, postID int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.ExpiredPostScore{}, "post_id = ?", postID)
	
	return errors.Wrap(res.Error, "mysql:DeletePostExpiredScoresByPostID")
}

