package logic

import (
	"bluebell/algorithm"
	"bluebell/dao/bleve"
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
	"strings"

	"strconv"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var hotpostGrp singleflight.Group
var postDetailGrp singleflight.Group
var postIDsGrp singleflight.Group
var postListGrp singleflight.Group
var postVoteNumGrp singleflight.Group

func CreatePost(post *models.Post) error {
	// mysql 持久化
	if err := mysql.CreatePost(post); err != nil {
		return err
	}

	// redis 缓存
	if err := redis.SetPost(post.PostID, post.CommunityID); err != nil {
		return err
	}

	doc := models.PostDoc{
		PostID:  post.PostID,
		Title:   utils.Substr(post.Title, 0, 64),    // 只索引前 64 个字符
		Content: utils.Substr(post.Content, 0, 256), // 只索引前 256 个字符
	}
	var err error
	if viper.GetBool("elasticsearch.enable") {
		doc.CreatedAt = models.Time(time.Now())
		err = elasticsearch.CreatePost(&doc)
	}
	if viper.GetBool("bleve.enable") {
		doc.CreatedAt = time.Now()
		err = bleve.CreatePost(&doc)
	}
	return errors.Wrap(err, "logic:CreatePost: index post doc")
}

func GetPostDetailByID(id int64, needIncrView bool) (detail *models.PostDTO, err error) {
	if needIncrView {
		if err := localcache.IncrView(objects.ObjPost, id, 1); err != nil {
			logger.Warnf("logic:GetPostDetailByID: IncrView failed(post)")
		}
	}

	cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, id) // 用于获取 local cache 的 key
	postCache, err := localcache.GetLocalCache().Get(cacheKey)
	if err == nil { // 本地缓存命中
		return postCache.(*models.PostDTO), nil
	}

	idStr := strconv.FormatInt(id, 10)

	timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
	rps := viper.GetInt("service.rps")
	interval := time.Second / time.Duration(rps)
	_detail, err := utils.SfDoWithTimeout(&postDetailGrp, idStr, timeout, interval, func() (any, error) {
		return mysql.SelectPostDetailByID(id)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bluebell.ErrNoSuchPost
		}
		return nil, errors.Wrap(err, "logic:GetPostDetailByID: SelectPostDetailByID")
	}
	detail = _detail.(*models.PostDTO)
	detail.VoteNum, err = redis.GetPostVoteNum(idStr)

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
	sfkey := fmt.Sprintf("%v_%v_%v_%v", params.Keyword, params.OrderBy, params.PageNum, params.PageSize)
	timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
	rps := viper.GetInt("service.rps")
	interval := time.Second / time.Duration(rps)

	ret, err := utils.SfDoWithTimeout(&postIDsGrp, sfkey, timeout, interval, func() (any, error) {
		var postIDs []string
		var err error
		total := 0
		if params.OrderBy == "time" {
			postIDs, total, err = elasticsearch.GetPostIDsByKeywordOrderByTime(params)
		} else if params.OrderBy == "correlation" {
			postIDs, total, err = elasticsearch.GetPostIDsByKeywordOrderByCorrelation(params)
		}
		return ReturnValueFromSearch{
			PostIDs: postIDs,
			Total:   total,
		}, err
	})

	if err != nil {
		return nil, 0, errors.Wrap(err, "logic:GetPostListByKeyword2:elasticsearch")
	}
	postIDs := ret.(ReturnValueFromSearch).PostIDs
	total := ret.(ReturnValueFromSearch).Total

	list, err := GetPostListByIDs(postIDs)
	return list, total, err
}

// 使用 bleve 实现的搜索
func GetPostListByKeyword(params *models.ParamPostListByKeyword) ([]*models.PostDTO, int, error) {
	sfkey := fmt.Sprintf("%v_%v_%v_%v", params.Keyword, params.OrderBy, params.PageNum, params.PageSize)
	timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
	rps := viper.GetInt("service.rps")
	interval := time.Second / time.Duration(rps)

	ret, err := utils.SfDoWithTimeout(&postIDsGrp, sfkey, timeout, interval, func() (any, error) {
		postIDs, total, err := bleve.GetPostIDsByKeyword(params)
		return ReturnValueFromSearch{
			PostIDs: postIDs,
			Total:   total,
		}, err
	})

	if err != nil {
		return nil, 0, errors.Wrap(err, "logic:GetPostListByKeyword2:bleve")
	}
	postIDs := ret.(ReturnValueFromSearch).PostIDs
	total := ret.(ReturnValueFromSearch).Total

	list, err := GetPostListByIDs(postIDs)
	return list, total, err
}

/*
*

	根据 postIDs 获取帖子列表
	返回的 posts 的 content 不一定是完整的
*/
func GetPostListByIDs(postIDs []string) ([]*models.PostDTO, error) {
	// 创建映射用于最后的结果组装
	resultMap := make(map[string]*models.PostDTO, len(postIDs))
	missPostIDs := []string{} // 记录缓存未命中的 ID

	// 先查 local cache
	for _, postID := range postIDs {
		cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, postID)
		if postDetail, err := localcache.GetLocalCache().Get(cacheKey); err == nil {
			resultMap[postID] = postDetail.(*models.PostDTO)
		} else {
			missPostIDs = append(missPostIDs, postID)
		}

		// 递增访问量
		id, _ := strconv.ParseInt(postID, 10, 64)
		localcache.IncrView(objects.ObjPost, id, 1)
	}
	// 在 mysql 中查询缓存未命中的 post list
	if len(missPostIDs) != 0 {
		sfkey := strings.Join(missPostIDs, "_")
		timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
		interval := time.Second / time.Duration(viper.GetInt("service.rps"))

		_missPostList, err := utils.SfDoWithTimeout(&postListGrp, sfkey, timeout, interval, func() (any, error) {
			return mysql.SelectPostListByPostIDs(missPostIDs)
		})

		if err != nil {
			return nil, errors.Wrap(err, "logic:GetPostListByIDs: SelectPostListByPostIDs")
		}
		missPostList := _missPostList.([]*models.PostDTO)

		// 获取过期帖子 ID
		expiredPostIDs := make([]string, 0)
		for i := 0; i < len(missPostList); i++ {
			if missPostList[i].Status == 1 {
				expiredPostIDs = append(expiredPostIDs, strconv.FormatInt(missPostList[i].PostID, 10))
			}
		}
		var voteNums []int64
		// 在 MySQL 中查询过期帖子的投票数
		if len(expiredPostIDs) != 0 {
			sfkey := strings.Join(expiredPostIDs, "_")
			timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
			rps := viper.GetInt("service.rps")
			interval := time.Second / time.Duration(rps)

			_voteNums, err := utils.SfDoWithTimeout(&postVoteNumGrp, sfkey, timeout, interval, func() (any, error) {
				return mysql.SelectPostVoteNumsByIDs(expiredPostIDs)
			})
			if err != nil {
				return nil, err
			}
			voteNums = _voteNums.([]int64)
		}

		// Assembling the resultMap
		for _, post := range missPostList {
			postIDStr := strconv.FormatInt(post.PostID, 10)
			resultMap[postIDStr] = post
		}

		if len(expiredPostIDs) != len(voteNums) {
			logger.Warnf("logic:GetPostListByIDs: len(expiredPostIDs) != len(voteNums)")
		} else {
			for idx, postID := range expiredPostIDs {
				resultMap[postID].VoteNum = voteNums[idx]
			}
		}
	}

	// 结合 Redis 计票数逻辑
	voteNumsFromRedis, err := redis.GetPostUpVoteNums(missPostIDs)
	if err != nil {
		return nil, err
	}

	// 为每个 post 添加分数字段
	for i, postID := range missPostIDs {
		resultMap[postID].VoteNum += voteNumsFromRedis[i]
	}

	// 构造最终返回的 post list
	list := make([]*models.PostDTO, 0, len(resultMap))
	for _, postID := range postIDs {
		if post, exists := resultMap[postID]; exists {
			list = append(list, post)
		} else {
			return nil, errors.Wrap(bluebell.ErrInternal, "logic:GetPostListByIDs: post not found")
		}
	}

	return list, nil
}

func GetPostListByAuthorID(params models.ParamUserPostList) (int, []*models.PostDTO, error) {
	start := (params.PageNum - 1) * params.PageSize

	postList, err := mysql.SelectPostsByAuthorID(params.UserID, start, params.PageSize)
	if err != nil {
		return 0, nil, errors.Wrap(err, "logic:GetPostListByAuthorID: SelectPostsByAuthorID")
	}
	total, err := mysql.SelectPostCountByAuthorID(params.UserID)
	return total, postList, errors.Wrap(err, "logic:GetPostListByAuthorID: SelectPostCountByAuthorID")
}

func GetHotPostList() ([]*models.PostDTO, error) {
	posts, err := localcache.GetLocalCache().Get("hotposts")
	if err != nil { // cache miss
		timeout := time.Second * time.Duration(viper.GetInt("service.timeout"))
		rps := viper.GetInt("service.rps")
		interval := time.Second / time.Duration(rps)

		res, err := utils.SfDoWithTimeout(&hotpostGrp, "hotpost", time.Duration(timeout), interval, func() (any, error) {
			// 热榜访问次数一定很高，如果 cache miss，需要立即重建
			return rebuild.RebuildPostHotList()
		})
		if err != nil {
			return nil, errors.Wrap(err, "logic:GetHotPostList: RebuildPostHotList")
		}
		return res.([]*models.PostDTO), nil
	}

	return posts.([]*models.PostDTO), nil
}

func RemovePost(userID int64, params models.ParamPostRemove) error {
	// 鉴权
	// 1. 获取 Post 的元数据（author_id、status）
	post, err := mysql.SelectPostDetailByID(params.PostID)
	if err != nil {
		return errors.Wrap(err, "logic:RemovePost: SelectPostDetailByID")
	}
	// 2. 判断 user_id 与 author_id 是否相等
	// 后续可以引入管理员
	if userID != post.UserID {
		return bluebell.ErrForbidden
	}

	// 删除 Post
	// 事务删除
	// 判断 status
	tx := mysql.GetDB().Begin()
	if err := mysql.DeletePostDetailByPostID(tx, post.PostID); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "logic:RemovePost: DeletePostDetailByPostID")
	}
	// 如果为 0，说明帖子没有过期，删除 post 表对应记录即可
	// 如果为 1，说明帖子过期，除了删除 post 表对应记录，还要删除 expired_post_scores 对应记录
	if post.Status == 1 {
		if err := mysql.DeletePostExpiredScoresByPostID(tx, post.PostID); err != nil {
			tx.Rollback()
			return errors.Wrap(err, "logic:RemovePost: DeletePostExpiredScoresByPostID")
		}
	}
	tx.Commit()

	// 判断 status，如果为 0，还要删除 redis 中的相关记录
	// 删除失败，简单处理如下：重试 5 次，每次间隔时间从 1s 开始指数增加
	// 后续可以引入消息队列，可以重新入队
	if post.Status == 0 {
		postIDStr := strconv.FormatInt(post.PostID, 10)
		tmp := []string{postIDStr}

		success := make([]bool, 4)
		duration := 1
		retry := 0
		maxRetry := 5

		for ; retry < maxRetry; retry++ {
			if retry > 0 {
				time.Sleep(time.Second * time.Duration(duration))
			}
			// 删除 score
			if !success[0] {
				if err = redis.DeletePostScores(tmp); err != nil {
					continue
				}
				success[0] = true
			}
			// 删除 post_time
			if !success[1] {
				if err = redis.DeletePostTimes(tmp); err != nil {
					continue
				}
				success[1] = true
			}
			// 删除 voted:post_id
			if !success[2] {
				if err = redis.DeletePostVotedNums(tmp); err != nil {
					continue
				}
				success[2] = true
			}
			if !success[3] {
				if err = redis.DeletePostInCommunity(post.CommunityID, tmp); err != nil {
					continue
				}
				success[3] = true
			}
			break
		}

		if retry == maxRetry {
			return errors.Wrap(err, "logic:RemovePost: Remove post metadata failed")
		}
	}

	if viper.GetBool("bleve.enable") {
		// 删 bleve 搜索引擎中的索引
		err = bleve.DeletePost(params.PostID)
		if err != nil {
			logger.Errorf("remove post from bleve failed, reason: %v", err.Error())
		}
	}
	if viper.GetBool("elasticsearch.enable") {
		// 删 elasticsearch 搜索引擎中的索引
		err = elasticsearch.DeletePost(params.PostID)
		if err != nil {
			logger.Errorf("remove post from elasticsearch failed, reason: %v", err.Error())
		}
	}

	// 删除本地缓存
	cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, post.PostID)
	localcache.GetLocalCache().Remove(cacheKey)
	return nil
}

type ReturnValueFromSearch struct {
	Total   int
	PostIDs []string
}
