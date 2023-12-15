package workers

import (
	"bluebell/dao/localcache"
	"bluebell/logger"
	"bluebell/logic"
	"bluebell/objects"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/viper"
)

// 需要解决根评论错位的问题

// 刷新热点帖子
func RefreshPostHotSpot() {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	waitTime := 0 * time.Second
	size := viper.GetInt("service.hot_spot.size_for_post")

	go func() {
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			// 从 redis 中获取前 size 条 view 的帖子 id
			postIDs, err := localcache.GetTopKObjectIDByViews(objects.ObjPost, size)
			if !checkError(err, &waitTime) {
				continue
			}

			// 获取帖子元数据
			for _, postID := range postIDs {
				post, err := logic.GetPostDetailByID(postID, false)
				if err != nil {
					logger.Warnf("workers:RefreshPostHotSpot: GetPostDetailByID failed")
					continue
				}

				// 添加帖子元数据到 local cache
				cacheKey := fmt.Sprintf("%v_%v", objects.ObjPost, postID)
				localcache.GetLocalCache().Set(cacheKey, post)
			}

			waitTime = refreshTime
			markAsExit()
		}
	}()
}

func RefreshCommentHotSpot()  {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	waitTime := 0 * time.Second
	size := viper.GetInt("service.hot_spot.size_for_comment")
	go func() {
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			// 获取要缓存的根评论 id
			commentIDs, err := localcache.GetTopKObjectIDByViews(objects.ObjComment, size)
			if !checkError(err, &waitTime) {
				continue
			}
			if len(commentIDs) == 0 {
				waitTime = refreshTime
				markAsExit()
				continue
			}

			// 获取 metadata
			// 注意，由于是在 local cache 获取的 commentID，需要按照 created_time（实际上就是 comment_id，雪花算法生成的） 升序排序
			// 才能保证数据的一致性（读 db 时，返回顺序是按照 commentID 升序排的）
			sort.Slice(commentIDs, func(i, j int) bool {
				return commentIDs[i] < commentIDs[j]
			})
			rootCommentDTOs, err := logic.GetCommentDetailByCommentIDs(true, false, commentIDs)

			if !checkError(err, &waitTime) {
				continue
			}

			// 获取 replies
			replies, err := logic.GetCommentDetailByCommentIDs(false, false, commentIDs)
			if !checkError(err, &waitTime) {
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

			waitTime = refreshTime
			markAsExit()
		}
	}()
}

func RemoveExpiredObjectView() {
	refreshTime := time.Second * time.Duration(viper.GetInt64("service.hot_spot.refresh_time"))
	waitTime := 0 * time.Second
	timeInterval := viper.GetInt64("service.hot_spot.time_interval")

	go func() {
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			// 从 bluebell:views 中获取过期的 view 的 otype_oid
			targetTimeStamp := time.Now().Unix() - timeInterval
			localcache.RemoveExpiredObjectView(targetTimeStamp)

			waitTime = refreshTime
			markAsExit()
		}
	}()
}
