package workers

import (
	"bluebell/dao/localcache"
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/models"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func PersistencePostScore(wg *sync.WaitGroup) {
	// 每 waitTime 检查一次，业务体量越小，检查时间可以越长
	waitTime := time.Second * time.Duration(viper.GetInt64("service.post.persistence_interval"))

	go func() {
		for {
			time.Sleep(waitTime)
			wg.Add(1)
			
			targetTimeStamp := time.Now().Unix() - viper.GetInt64("service.post.active_time") // Unix 返回的已经是 second！！！
			// 从 redis 中获取过期帖子的 ID，存放到一个切片中
			postIDs, err := redis.GetExpiredPostID(targetTimeStamp)
			if len(postIDs) == 0 { // 避免后续操作
				wg.Done()
				continue
			}
			
			if !checkError(err, &waitTime, wg) {
				continue
			}

			// 从 redis 中获取帖子分数
			postScores, err := redis.GetPostScores(postIDs)
			if !checkError(err, &waitTime, wg) {
				continue
			}

			// 从 redis 中获取 voteNums
			upVoteNums, err := redis.GetPostUpVoteNums(postIDs)
			if !checkError(err, &waitTime, wg) {
				continue
			}
			downVoteNums, err := redis.GetPostUpVoteNums(postIDs)
			if !checkError(err, &waitTime, wg) {
				continue
			}

			if len(postIDs) != len(postScores) || len(postScores) != len(upVoteNums) || len(upVoteNums) != len(downVoteNums) {
				checkError(errors.New("Unexpected length in persistence post scores"), &waitTime, wg)
				continue
			}

			// 组装数据
			expiredPosts := make([]models.ExpiredPostScore, 0, len(postIDs))
			for i := 0; i < len(postIDs); i++ {
				post_id, _ := strconv.ParseInt(postIDs[i], 10, 64)
				expiredPosts = append(expiredPosts, models.ExpiredPostScore{
					PostID:      post_id,
					PostScore:   postScores[i],
					PostVoteNum: upVoteNums[i] - downVoteNums[i],
				})
			}

			// 修改过期帖子的状态为 1
			tx := mysql.GetDB().Begin()
			if err := mysql.UpdatePostStatusByPostIDs(tx, 1, postIDs); !checkError(err, &waitTime, wg) {
				continue
			}

			// 将过期帖子的分数数据持久化到 MySQL
			if err := mysql.CreateExpiredPostScores(tx, expiredPosts); !checkError(err, &waitTime, wg) {
				continue
			}

			tx.Commit()
			logger.Infof("Persisted %d pieces of expired data from Redis to MySQL", len(postIDs))

			// 从 Redis 中删除帖子数据
			// 删除 score
			if err := redis.DeletePostScores(postIDs); !checkError(err, &waitTime, wg) {
				continue
			}
			// 删除 post_time
			if err := redis.DeletePostTimes(postIDs); !checkError(err, &waitTime, wg) {
				continue
			}
			// 删除 voted:post_id
			if err := redis.DeletePostVotedNums(postIDs); !checkError(err, &waitTime, wg) {
				continue
			}

			// 删除 community 中的 post
			communityIDs, err := mysql.SelectCommunityIDs()
			if !checkError(err, &waitTime, wg) {
				continue
			}

			for _, communityID := range communityIDs {
				redis.DeleteExpiredPostInCommunity(communityID, targetTimeStamp) // 使用同一个 targetTimeStamp，保证删除数据的一致性
			}
			logger.Infof("Removed %d pieces of expired data from Redis", len(postIDs))

			waitTime = time.Second * time.Duration(viper.GetInt64("service.post.persistence_interval"))
			wg.Done()
		}
	}()

	// 修改后，也要同步修改 logic 中的 post.go 的 GetPostDetailByID
}

func RefreshHotPost(wg *sync.WaitGroup)  {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_post_list.refresh_time"))
	size := viper.GetInt64("service.hot_post_list.size")

	go func() {
		for {
			wg.Add(1)
			
			postID, _, err := redis.GetPostIDs(1, size, "score")
			if !checkError(err, &refreshTime, wg)  {
				continue
			}

			hotPosts, err := logic.GetPostListByIDs(postID)
			if !checkError(err, &refreshTime, wg)  {
				continue
			}

			localcache.GetLocalCache().Set("hotposts", hotPosts)
			logger.Infof("Refreshed hot post list")
			wg.Done()
			time.Sleep(refreshTime)
		}
	}()
}
