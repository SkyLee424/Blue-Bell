package workers

import (
	"bluebell/dao/mysql"
	"bluebell/dao/redis"
	"bluebell/internal/utils"
	"bluebell/logger"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// 持久化评论的点赞数
func PersistenceCommentCount(like bool) {
	persistenceInterval := time.Second * time.Duration(viper.GetInt64("service.comment.count.persistence_interval"))
	waitTime := 0 * time.Second
	countExpireTime := time.Second * time.Duration(viper.GetInt64("service.comment.count.expire_time"))

	go func() {
	rootloop:
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			// 获取所有的 key
			pf := redis.KeyCommentLikeStringPF
			if !like {
				pf = redis.KeyCommentHateStringPF
			}
			keys, err := redis.GetKeys(pf + "*")
			if !checkError(err, &waitTime) {
				continue
			}

			// 筛选逻辑过期的 key，准备持久化
			expiredKeys, err := getExpiredKeys(keys, countExpireTime)
			if !checkError(err, &waitTime) {
				continue
			}
			// 不需要持久化
			if len(expiredKeys) == 0 {
				waitTime = persistenceInterval
				markAsExit()
				continue
			}

			// 获取过期评论的点赞（踩）数
			counts, err := redis.GetCommentLikeOrHateCountByKeys(expiredKeys)
			if !checkError(err, &waitTime) {
				continue
			}

			// 持久化
			tx := mysql.GetDB().Begin() // 事务更新
			field := "`like`"
			if !like {
				field = "hate"
			}
			for i := 0; i < len(expiredKeys); i++ {
				commentID, _ := strconv.ParseInt(utils.Substr(expiredKeys[i], len(redis.KeyCommentLikeStringPF), len(expiredKeys[i])), 10, 64)
				err := mysql.IncrCommentIndexCountField(tx, field, commentID, counts[i])
				if !checkError(err, &waitTime) {
					tx.Rollback()     // 有一个更新失败，回滚事务，稍后重新尝试
					continue rootloop // 注意，不是当前的循环，是最外层的循环
				}
			}
			tx.Commit()
			logger.Infof("workers:persistenceCountHelper: Persisted %d pieces of expired like(hate) counts from Redis to MySQL", len(counts))

			// 删除逻辑过期的 key（失败也不用马上重试，开销大）
			if err := redis.DelKeys(expiredKeys); err != nil {
				logger.Warnf("workers:persistenceCountHelper: Removed expired data from Redis failed, reason: %v", err.Error())
			} else {
				logger.Infof("workers:persistenceCountHelper: Removed %d pieces of expired data from Redis", len(expiredKeys))
			}

			waitTime = persistenceInterval
			markAsExit()
		}
	}()
}

// 持久化评论有哪些用户点赞
func PersistenceCommentCidUid(like bool) {
	persistenceInterval := time.Second * time.Duration(viper.GetInt64("service.comment.like_hate_user.persistence_interval"))
	waitTime := 0 * time.Second
	tmpStr := "service.comment.like_hate_user.like_expire_time"
	pf := redis.KeyCommentLikeSetPF
	if !like {
		tmpStr = "service.comment.like_hate_user.hate_expire_time"
		pf = redis.KeyCommentHateSetPF
	}
	expireTime := time.Second * time.Duration(viper.GetInt64(tmpStr))

	go func() {
	rootloop:
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			keys, err := redis.GetKeys(pf + "*")
			if !checkError(err, &waitTime) {
				continue
			}

			// 筛选逻辑过期的 key
			expiredKeys, err := getExpiredKeys(keys, expireTime)
			if !checkError(err, &waitTime) {
				continue
			}
			// 不需要持久化
			if len(expiredKeys) == 0 {
				waitTime = persistenceInterval
				markAsExit()
				continue
			}

			// 遍历
			for i := 0; i < len(expiredKeys); i++ {
				UserIDs, err := redis.GetSetMembersByKey(expiredKeys[i])
				if !checkError(err, &waitTime) {
					continue rootloop
				}

				tmpStr := utils.Substr(expiredKeys[i], len(redis.KeyCommentLikeSetPF), len(expiredKeys[i]))
				tmpStrArr := strings.Split(tmpStr, "_")
				commentID, _ := strconv.ParseInt(tmpStrArr[0], 10, 64)
				objID, _ := strconv.ParseInt(tmpStrArr[1], 10, 64)
				objType, _ := strconv.ParseInt(tmpStrArr[2], 10, 8)
				tx := mysql.GetDB().Begin()
				// 持久化
				for i := 0; i < len(UserIDs); i++ {
					userID, _ := strconv.ParseInt(UserIDs[i], 10, 64)
					err := mysql.CreateCommentLikeOrHateUser(tx, commentID, userID, objID, int8(objType), like)
					if !checkError(err, &waitTime) {
						tx.Rollback()
						continue rootloop // ?
					}
				}
				tx.Commit()
				logger.Infof("workers:persistenceCidUidHelper: Persisted %d pieces of expired cid_uid from Redis to MySQL", len(UserIDs))
			}

			// 删除逻辑过期的 key
			if err := redis.DelKeys(expiredKeys); err != nil {
				logger.ErrorWithStack(err)
			} else {
				logger.Infof("workers:persistenceCidUidHelper: Removed %d pieces of expired data from Redis", len(expiredKeys))
			}

			waitTime = persistenceInterval
			markAsExit()
		}
	}()
}

func RemoveCommentCidUidFromDB() {
	removeInterval := time.Second * time.Duration(viper.GetInt64("service.comment.like_hate_user.remove_interval"))
	waitTime := 0 * time.Second
	go func() {
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			commentIDStrs, err := redis.GetSetMembersByKey(redis.KeyCommentRemCidSet)
			if !checkError(err, &waitTime) {
				continue
			}

			// 不需要删除
			if len(commentIDStrs) == 0 {
				waitTime = removeInterval
				markAsExit()
				continue
			}

			commentIDs := make([]int64, len(commentIDStrs))
			for i := 0; i < len(commentIDStrs); i++ {
				commentIDs[i], _ = strconv.ParseInt(commentIDStrs[i], 10, 64)
			}
			err = mysql.DeleteCommentUserLikeMappingByCommentIDs(nil, commentIDs)
			if !checkError(err, &waitTime) {
				continue
			}

			logger.Infof("workers:RemoveCommentCidUid: Removed %d ciduid in mysql", len(commentIDs))

			err = redis.DelKeys([]string{redis.KeyCommentRemCidSet})
			if !checkError(err, &waitTime) {
				continue
			}

			logger.Infof("workers:RemoveCommentCidUid: Removed %d ciduid in redis", len(commentIDs))
			waitTime = removeInterval
			markAsExit()
		}
	}()
}

func RemoveCommentIndexFromRedis() {
	removeInterval := time.Second * time.Duration(viper.GetInt64("service.comment.index.remove_interval"))
	expireTime := time.Second * time.Duration(viper.GetInt64("service.comment.index.expire_time"))
	pattern := redis.KeyCommentIndexZSetPF + "*"

	removeLogicalExpiredKeysHelper(removeInterval, expireTime, pattern)
}

func RemoveCommentContentFromRedis() {
	removeInterval := time.Second * time.Duration(viper.GetInt64("service.comment.content.remove_interval"))
	expireTime := time.Second * time.Duration(viper.GetInt64("service.comment.content.expire_time"))
	pattern := redis.KeyCommentContentStringPF + "*"

	removeLogicalExpiredKeysHelper(removeInterval, expireTime, pattern)
}

func removeLogicalExpiredKeysHelper(removeInterval, logicalExpireTime time.Duration, pattern string) {
	waitTime := 0 * time.Second
	go func() {
		for {
			time.Sleep(waitTime)
			if checkIfExit() {
				return				
			}

			keys, err := redis.GetKeys(pattern)
			if !checkError(err, &waitTime) {
				continue
			}

			idleTimes, err := redis.GetKeysIdleTime(keys)
			if !checkError(err, &waitTime) {
				continue
			}
			if len(keys) != len(idleTimes) {
				checkError(errors.New("workers:removeLogicalExpiredKeysHelper: keys and idleTimes length not equal"),
					&waitTime)
				continue
			}

			expiredKeys := make([]string, 0, len(keys))
			for i := 0; i < len(idleTimes); i++ {
				if idleTimes[i] > logicalExpireTime {
					expiredKeys = append(expiredKeys, keys[i])
				}
			}
			if len(expiredKeys) == 0 { // 不需要删除
				waitTime = removeInterval
				markAsExit()
				continue
			}

			err = redis.DelKeys(expiredKeys)
			if !checkError(err, &waitTime) {
				continue
			}

			logger.Infof("workers:removeLogicalExpiredKeysHelper: Removed %d expired keys(%v) from redis", len(expiredKeys), pattern)
			waitTime = removeInterval
			markAsExit()
		}
	}()
}
