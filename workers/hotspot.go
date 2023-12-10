package workers

import (
	"bluebell/dao/localcache"
	"bluebell/dao/redis"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/objects"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// 刷新热点帖子
func RefreshPostHotSpot(wg *sync.WaitGroup) {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	size := viper.GetInt64("service.hot_spot.size_for_post")

	go func() {
		for {
			wg.Add(1)

			// 从 redis 中获取前 size 条 view 的帖子 id
			postIDs, _, err := redis.GetPostIDs(1, size, "views")
			if !checkError(err, &refreshTime, wg) {
				continue
			}

			// 获取帖子元数据
			for _, postIDStr := range postIDs {
				postID, _ := strconv.ParseInt(postIDStr, 10, 64)
				post, err := logic.GetPostDetailByID(postID, false)
				if err != nil {
					logger.Warnf("workers:RefreshPostHotSpot: GetPostDetailByID failed")
					continue
				}

				// 添加帖子元数据到 local cache
				cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, postID)
				localcache.GetLocalCache().Set(cacheKey, post)
			}

			// // for debug
			// all := localcache.GetLocalCache().GetALL(false)
			// for _, v := range all {
			// 	logger.("localcache: %v", v)
			// }
			wg.Done()
			time.Sleep(refreshTime)
		}
	}()
}

func RemoveExpiredObjectView(wg *sync.WaitGroup) {
	waitTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	timeInterval := viper.GetInt64("service.hot_spot.time_interval")

	go func() {
		for {
			wg.Add(1)

			// 从 bluebell:views 中获取过期的 view 的 otype_oid
			targetTimeStamp := time.Now().Unix() - timeInterval
			expiredMembers, err := redis.GetZSetMembersRangeByScore(redis.KeyViewCreatedTimeZSet, "0", fmt.Sprintf("%v", targetTimeStamp))
			if !checkError(err, &waitTime, wg) {
				continue
			}

			for _, expiredMember := range expiredMembers {
				tmp := strings.Split(expiredMember, "_")
				objType, err := strconv.ParseInt(tmp[0], 10, 64)
				if !checkError(err, &waitTime, wg) {
					continue
				}
				objID, err := strconv.ParseInt(tmp[1], 10, 64)
				if !checkError(err, &waitTime, wg) {
					continue
				}
				if objType == objects.ObjPost {
					redis.ZSetRem(redis.KeyPostViewsZset, objID)
				}
				redis.ZSetRem(redis.KeyViewCreatedTimeZSet, expiredMember)
			}

			wg.Done()
			time.Sleep(waitTime)
		}
	}()
}
