package workers

import (
	"bluebell/dao/localcache"
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/objects"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// 需要解决根评论错位的问题

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

func RefreshCommentHotSpot(wg *sync.WaitGroup)  {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	size := viper.GetInt64("service.hot_spot.size_for_comment")
	go func() {
		root:
		for {
			wg.Add(1)

			// 获取要缓存的根评论 id
			commentIDStrs, err := redis.GetZSetMembersRangeByIndex(redis.KeyCommentViewZset, 0, size, true)
			if !checkError(err, &refreshTime, wg) {
				continue
			}
			if len(commentIDStrs) == 0 {
				wg.Done()
				time.Sleep(refreshTime)
				continue
			}

			// 获取 metadata
			commentIDs := make([]int64, len(commentIDStrs))
			for idx, commentIDStr := range commentIDStrs {
				commentIDs[idx], err = strconv.ParseInt(commentIDStr, 10, 64)
				if !checkError(err, &refreshTime, wg) {
					continue root
				}
			}
			// 注意，由于是在 bluebell:view:comment 获取的 commentID，需要按照升序排序，才能保证数据的一致性（读 db 时，返回顺序是按照 commentID 升序排的）
			sort.Slice(commentIDs, func(i, j int) bool {
				return commentIDs[i] < commentIDs[j]
			})
			rootCommentDTOs, err := logic.GetCommentDetailByCommentIDs(true, false, commentIDs)

			if !checkError(err, &refreshTime, wg) {
				continue
			}

			// 获取 replies
			replies, err := logic.GetCommentDetailByCommentIDs(false, false, commentIDs)
			if !checkError(err, &refreshTime, wg) {
				continue
			}

			// 写 local cache
			// root comment's metadata
			for i := 0; i < len(rootCommentDTOs); i++ {
				cacheKey := fmt.Sprintf("%v_%v_metadata", objects.ObjComment, rootCommentDTOs[i].CommentID)
				if err := localcache.GetLocalCache().Set(cacheKey, rootCommentDTOs[i]); err != nil {
					logger.Warnf("Add root comment metadata to local cache failed, reason: %v", err.Error())
				}

			}
			replyLists := make(map[int64][]int64)
			for _, reply := range replies {
				rootCommentID := reply.Root
				_, ok := replyLists[rootCommentID];
				if !ok {
					replyLists[rootCommentID] = make([]int64, 0)	
				}
				replyLists[rootCommentID] = append(replyLists[rootCommentID], reply.CommentID)

				// sub comment's metadata
				cacheKey := fmt.Sprintf("%v_%v_metadata", objects.ObjComment, reply.CommentID)
				if err := localcache.GetLocalCache().Set(cacheKey, reply); err != nil {
					logger.Warnf("Add root reply comment metadata to local cache failed, reason: %v", err.Error())
				}
			}
			// replies
			// 注意：遍历 rootCommentDTOs，而不是根据 replyLists 来写缓存
			// 如果有评论没有二级评论，会造成缓存穿透
			// for rootCommentID, replyList := range replyLists {
			// 	cacheKey := fmt.Sprintf("%v_%v_replies", objects.ObjComment, rootCommentID)
			// 	if err := localcache.GetLocalCache().Set(cacheKey, replyList); err != nil {
			// 		logger.Warnf("Add comment reply list to local cache failed, reason: %v", err.Error())
			// 	}
			// }
			for _, rootCommentDTO := range rootCommentDTOs {
				rootCommentID := rootCommentDTO.CommentID
				cacheKey := fmt.Sprintf("%v_%v_replies", objects.ObjComment, rootCommentID)
				replyList := replyLists[rootCommentID]
				// 即使 replyList 是 nil，也要写入！
				if err := localcache.GetLocalCache().Set(cacheKey, replyList); err != nil {
					logger.Warnf("Add comment reply list to local cache failed, reason: %v", err.Error())
				}
			}

			wg.Done()
			time.Sleep(refreshTime)
		}
	}()
}

func RemoveExpiredObjectView(wg *sync.WaitGroup) {
	waitTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	timeInterval := viper.GetInt64("service.hot_spot.time_interval")

	go func() {
		root:
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
				if len(tmp) != 2 { // 检查一下长度
					checkError(bluebell.ErrInternal, &waitTime, wg)
					continue root
				}
				objType, err := strconv.ParseInt(tmp[0], 10, 64)
				if !checkError(err, &waitTime, wg) {
					continue root
				}
				objID, err := strconv.ParseInt(tmp[1], 10, 64)
				if !checkError(err, &waitTime, wg) {
					continue root
				}

				var remKey string
				switch objType {
				case objects.ObjPost:
					remKey = redis.KeyPostViewsZset
				case objects.ObjComment:
					remKey = redis.KeyCommentViewZset
				}

				redis.ZSetRem(remKey, objID)
				redis.ZSetRem(redis.KeyViewCreatedTimeZSet, expiredMember)
			}

			wg.Done()
			time.Sleep(waitTime)
		}
	}()
}
