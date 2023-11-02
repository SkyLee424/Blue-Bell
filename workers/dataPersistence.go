package workers

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	"bluebell/logger"
	"bluebell/models"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func PersistenceScore(wg *sync.WaitGroup) {
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
			voteNums, err := redis.GetPostVoteNums(postIDs)
			if !checkError(err, &waitTime, wg) {
				continue
			}

			if len(postIDs) != len(postScores) || len(postScores) != len(voteNums) {
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
					PostVoteNum: voteNums[i],
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

// 帖子的业务逻辑：
// 在过期前，可以正常的投票，也可以在主页面看到该帖子的信息
// 在过期后，不允许投票，在主页也不可以看到帖子的信息
// 后续前端添加一个根据 ID 搜索的逻辑，通过这个方式，允许获得过期帖子的信息

// 检查错误，如果有错误：
//
// 1. 输出日志
// 2. 修改 waitTime 为较小值，尽快重试
// 3. 调用 waitGroup.Done
func checkError(err error, waitTime *time.Duration, wg *sync.WaitGroup) bool {
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.ErrorWithStack(err)
		*waitTime = time.Second * 10 // 10 s 后再次尝试获取
		wg.Done()
		return false
	}
	return true
}
