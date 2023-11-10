package logic

import (
	"bluebell/dao/elasticsearch"
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/models"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func CreatePost(post *models.Post) error {
	// mysql 持久化
	if err := mysql.CreatePost(post); err != nil {
		return err
	}

	// redis 缓存
	if err := redis.SetPost(post.PostID, post.CommunityID); err != nil {
		return err
	}

	// elasticsearch 索引文档
	elasticsearch.CreatePost(&models.PostDoc{
		PostID:    post.PostID,
		Title:     utils.Substr(post.Title, 0, 64),    // 只索引前 64 个字符
		Content:   utils.Substr(post.Content, 0, 256), // 只索引前 256 个字符
		CreatedAt: models.Time(time.Now()),
	})

	return nil
}

func GetPostDetailByID(id int64) (detail *models.PostDTO, err error) {
	detail, err = mysql.SelectPostDetailByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bluebell.ErrNoSuchPost
		}
		return nil, err
	}

	detail.VoteNum, err = redis.GetPostVoteNum(strconv.FormatInt(id, 10))

	return detail, err
}

// 推荐阅读
// 基于用户投票的相关算法：http://www.ruanyifeng.com/blog/algorithm/

// 本项目使用简化版的投票分数
// 时间戳
// 投一票就加432分   86400(1d)/432=200  --> 200张赞成票可以给你的帖子续一天

/* 投票的几种情况：
direction=1时，有两种情况：
	1. 之前没有投过票，现在投赞成票    --> 更新分数和投票记录
	2. 之前投反对票，现在改投赞成票    --> 更新分数和投票记录
direction=0时，有两种情况：
	1. 之前投过赞成票，现在要取消投票  --> 更新分数和投票记录
	2. 之前投过反对票，现在要取消投票  --> 更新分数和投票记录
direction=-1时，有两种情况：
	1. 之前没有投过票，现在投反对票    --> 更新分数和投票记录
	2. 之前投赞成票，现在改投反对票    --> 更新分数和投票记录

投票的限制：
每个贴子自发表之日起一个星期之内允许用户投票，超过一个星期就不允许再投票了。
	1. 到期之后将redis中保存的赞成票数及反对票数存储到mysql表中
	2. 到期之后删除那个 KeyPostVotedZSetPF
*/

func VoteForPost(user_id, post_id int64, direction int8) error {
	// 获取发布时间
	publishTime, err := redis.GetPostPublishTimeByPostID(strconv.FormatInt(post_id, 10))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return bluebell.ErrNoSuchPost
		}
		return err
	}
	timeDiff := float64(time.Now().Unix()) - publishTime
	activeTime := viper.GetInt64("service.post.active_time") // 读取配置，这里是一周
	if timeDiff > float64(activeTime) {
		return bluebell.ErrVoteTimeExpire
	}

	// 获取帖子原来分数
	score, err := redis.GetPostScore(post_id)
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	// 获取用户原来的投票选项
	oldDirection, err := redis.GetUserPostDirection(post_id, user_id)
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	// 不需要修改
	if oldDirection == direction {
		return nil
	}

	// 计算修改后的分数
	newScore := int64(direction-oldDirection)*432 + int64(score)

	// 保存分数
	if err := redis.SetPostScore(post_id, newScore); err != nil {
		return err
	}

	// 保存用户操作
	return redis.SetUserPostDirection(post_id, user_id, direction)
}

func GetAllPostList(params *models.ParamPostList) ([]*models.PostDTO, error) {
	// 在 redis 中查询 posts 的 id
	var postIDs []string
	var err error
	if params.CommunityID == -1 {
		postIDs, err = redis.GetPostIDs(params.PageNum, params.PageSize, params.OrderBy) // 默认查询所有 community 的 post
	} else {
		postIDs, err = redis.GetPostIDsByCommunity(params.PageNum, params.PageSize, params.OrderBy, params.CommunityID)
	}

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, err
		}
		return nil, errors.Wrap(err, "get post_id lists from redis") // 避免没有必要的查询
	}

	return GetPostListByIDs(postIDs)
}

// 难点：
// 1. 怎么高效地在 MySQL 中模糊查询？
// 2. 怎么建立索引？
// 3. 全文模糊查询如何实现？
func GetPostListByKeyword(params *models.ParamPostListByKeyword) ([]*models.PostDTO, error) {
	// 在 mysql post 中模糊查询标题包含 keyword 的帖子 ID，全文索引
	postIDs_0, err := mysql.DemoSelectPostIDsByTitleKeyword(params.PageNum, params.PageSize, params.Keyword)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	// 在 mysql post 中模糊查询正文包含 keyword 的帖子 ID，全文索引
	postIDs_1, err := mysql.DemoSelectPostIDsByContentKeyword(params.PageNum, params.PageSize, params.Keyword)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	// 组合 ID（注意去重）
	uniquePostIDs := make(map[string]bool)
	for _, id := range postIDs_0 {
		uniquePostIDs[id] = true
	}
	for _, id := range postIDs_1 {
		uniquePostIDs[id] = true
	}
	finalPostIDs := make([]string, 0, len(uniquePostIDs))
	for id := range uniquePostIDs {
		finalPostIDs = append(finalPostIDs, id)
	}

	// 根据 ID 获取帖子列表
	return GetPostListByIDs(finalPostIDs)

}

func GetPostListByKeyword2(params *models.ParamPostListByKeyword) ([]*models.PostDTO, error) {
	postIDs := make([]string, 0)
	var err error

	if params.OrderBy == "time" {
		postIDs, err = elasticsearch.GetPostIDsByKeywordOrderByTime(params)
	} else if params.OrderBy == "correlation" {
		postIDs, err = elasticsearch.GetPostIDsByKeywordOrderByCorrelation(params)
	}
	if err != nil {
		return nil, errors.Wrap(err, "GetPostListByKeyword2: ")
	}

	return GetPostListByIDs(postIDs)
}

func GetPostListByIDs(postIDs []string) ([]*models.PostDTO, error) {
	// 在 mysql 中查询 post list
	list, err := mysql.SelectPostListByPostIDs(postIDs)
	if err != nil {
		return nil, errors.Wrap(err, "get post list from mysql")
	}

	// 获取过期帖子 ID
	expiredPostIDs := make([]string, 0)
	for i := 0; i < len(list); i++ {
		if list[i].Status == 1 {
			expiredPostIDs = append(expiredPostIDs, strconv.FormatInt(list[i].PostID, 10))
		}
	}

	// 在 mysql 中查询每个过期 post 的投票数
	voteNumsFromMySQL := make([]int64, 0)
	if len(expiredPostIDs) != 0 {
		voteNumsFromMySQL, err = mysql.SelectPostVoteNumsByIDs(expiredPostIDs)
		if err != nil {
			return nil, err
		}
	}

	// 在 redis 中查询每个 post 的投票数（如果帖子过期，查询仍会成功，且 votenum 为 0）
	voteNumsFromRedis, err := redis.GetPostVoteNums(postIDs)
	if err != nil {
		return nil, err
	}

	// not expected
	if len(voteNumsFromRedis) != len(list) {
		return nil, bluebell.ErrInternal
	}

	// 为每个 post 添加分数字段
	cur := 0
	for i := 0; i < len(list); i++ {
		if list[i].Status == 0 { // 没有过期，使用 redis 的数据
			list[i].VoteNum = voteNumsFromRedis[i]
		} else { // 过期，使用 mysql 的数据
			list[i].VoteNum = voteNumsFromMySQL[cur]
			cur++
		}
	}

	// 返回
	return list, nil
}
