package logic

import (
	"bluebell/algorithm"
	"bluebell/dao/elasticsearch"
	"bluebell/dao/localcache"
	"bluebell/dao/mysql"
	"bluebell/dao/rebuild"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"bluebell/logger"
	"bluebell/models"
	"bluebell/objects"
	"fmt"
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

func GetPostDetailByID(id int64, needIncrView bool) (detail *models.PostDTO, err error) {
	postIDStr := strconv.FormatInt(id, 10)
	if needIncrView {
		newMember, err := redis.ZSetIncrBy(redis.KeyPostViewsZset, postIDStr, 1) // 增加一次访问量
		if err != nil {
			logger.Warnf("logic:GetPostDetailByID: ZSetIncrBy failed(incr post view)")
		} else if newMember == true { // 如果是新创建的 member，在 redis 中记录创建时间，用于统计一个时间段的 view
			member := fmt.Sprintf("%v_%v", objects.ObjPost, postIDStr)
			if err := redis.ZSetAdd(redis.KeyPostViewsZset, member, float64(time.Now().Unix())); err != nil {
				logger.Warnf("logic:GetPostDetailByID: SetPostViewCreateTime failed")
				// 应该保证事务一致性原则（回滚 incr 操作）
				// 这里简单处理，不考虑回滚失败
				redis.ZSetIncrBy(redis.KeyPostViewsZset, postIDStr, -1)
			}
		}
	}

	cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, id) // 用于获取 local cache 的 key
	postCache, err := localcache.GetLocalCache().Get(cacheKey)
	if err == nil { // 本地缓存命中
		return postCache.(*models.PostDTO), nil
	}

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

	// 保存用户操作
	if err := redis.SetUserPostDirection(post_id, user_id, direction); err != nil {
		return errors.Wrap(err, "logic:VoteForPost: SetUserPostDirection")
	}

	postIDStr := fmt.Sprintf("%d", post_id)
	upVoteNum, err := redis.GetPostUpVoteNums([]string{postIDStr})
	if err != nil {
		return errors.Wrap(err, "logic:VoteForPost: GetPostUpVoteNums")
	}
	downVoteNum, err := redis.GetPostDownVoteNums([]string{postIDStr})
	if err != nil {
		return errors.Wrap(err, "logic:VoteForPost: GetPostUpVoteNums")
	}

	// 更新帖子的分数
	newScore := algorithm.GetPostScoreByReddit(int64(publishTime), upVoteNum[0]-downVoteNum[0])

	// 判断是否需要更新 local cache
	cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, postIDStr)
	postInCache, err := localcache.GetLocalCache().Get(cacheKey)
	if err == nil { // cache hit，更新 local cache
		post := postInCache.(*models.PostDTO)
		post.VoteNum = upVoteNum[0]
		localcache.GetLocalCache().Set(cacheKey, post)
	}
	return errors.Wrap(redis.SetPostScore(post_id, newScore), "logic:VoteForPost: SetPostScore")

	// // 获取帖子原来分数
	// score, err := redis.GetPostScore(post_id)
	// if err != nil && !errors.Is(err, redis.Nil) {
	// 	return err
	// }

	// // 计算修改后的分数
	// newScore := int64(direction-oldDirection)*432 + int64(score)

	// // 保存分数
	// if err := redis.SetPostScore(post_id, newScore); err != nil {
	// 	return err
	// }

}

func GetAllPostList(params *models.ParamPostList) ([]*models.PostDTO, int, error) {
	// 在 redis 中查询 posts 的 id
	var postIDs []string
	var err error
	var total int
	if params.CommunityID == -1 {
		postIDs, total, err = redis.GetPostIDs(params.PageNum, params.PageSize, params.OrderBy) // 默认查询所有 community 的 post
	} else {
		postIDs, total, err = redis.GetPostIDsByCommunity(params.PageNum, params.PageSize, params.OrderBy, params.CommunityID)
	}

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, total, nil
		}
		return nil, 0, errors.Wrap(err, "get post_id lists from redis") // 避免没有必要的查询
	}

	// 分页
	list, err := GetPostListByIDs(postIDs)
	return list, total, err
}

func GetPostListByKeyword2(params *models.ParamPostListByKeyword) ([]*models.PostDTO, int, error) {
	postIDs := make([]string, 0)
	var err error
	var total int

	if params.OrderBy == "time" {
		postIDs, total, err = elasticsearch.GetPostIDsByKeywordOrderByTime(params)
	} else if params.OrderBy == "correlation" {
		postIDs, total, err = elasticsearch.GetPostIDsByKeywordOrderByCorrelation(params)
	}
	if err != nil {
		return nil, 0, errors.Wrap(err, "logic:GetPostListByKeyword2:elasticsearch")
	}

	list, err := GetPostListByIDs(postIDs)
	return list, total, err
}

func GetPostListByIDs(postIDs []string) ([]*models.PostDTO, error) {
	list := make([]*models.PostDTO, len(postIDs))
	missPostIDs := make([]string, 0, len(postIDs))
	// 先查 local cache
	for idx, postID := range postIDs {
		cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, postID)
		postDetail, err := localcache.GetLocalCache().Get(cacheKey)
		if err == nil {
			list[idx] = postDetail.(*models.PostDTO)
		} else { // local cache miss
			missPostIDs = append(missPostIDs, postID)
		}
	}

	// 在 mysql 中查询缓存未命中的 post list
	if len(missPostIDs) != 0 {
		missPostList, err := mysql.SelectPostListByPostIDs(missPostIDs)
		if err != nil {
			return nil, errors.Wrap(err, "get post list from mysql")
		}
		// 组装数据
		idx := 0
		for i := 0; i < len(list); i++ {
			if list[i] == nil {
				list[i] = missPostList[idx]
				idx++
			}
		}
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
	var err error
	if len(expiredPostIDs) != 0 {
		voteNumsFromMySQL, err = mysql.SelectPostVoteNumsByIDs(expiredPostIDs)
		if err != nil {
			return nil, err
		}
	}

	// 在 redis 中查询每个 post 的投票数（如果帖子过期，查询仍会成功，且 votenum 为 0）
	voteNumsFromRedis, err := redis.GetPostUpVoteNums(postIDs)
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

func GetHotPostList() ([]*models.PostDTO, error) {
	posts, err := localcache.GetLocalCache().Get("hotposts")
	if err != nil { // cache miss
		// 热榜访问次数一定很高，如果 cache miss，需要立即重建
		return rebuild.RebuildPostHotList()
	}

	return posts.([]*models.PostDTO), nil
}
