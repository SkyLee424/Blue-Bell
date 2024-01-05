package kafka

import (
	"bluebell/dao/localcache"
	"bluebell/dao/mysql"
	"bluebell/logger"
	"context"
	"encoding/json"
	"fmt"

	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

/*
	kafka-consumer 的基本操作
*/

// 串行消费模型
func basicSerialConsumerWork(ch chan int, consumer *kafka.Reader) {
	defer wg.Done()
	defer consumer.Close() // 先 close，再 done
	batchSize := 10        // 一批消息的大小，取决于 db 能承受的并发度
	timeout := 5000        // 每 5s 再次尝试 fetch，主要是检测是否应该退出循环使用，时间设置不宜过短

rootloop:
	for {
		// 检查是否应该退出循环
		select {
		case <-ch:
			fmt.Println("exit.")
			break rootloop
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(timeout))
		defer cancel()

		msgs, err := fetchMessages(ctx, consumer, batchSize)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) { // 其它错误
				log.Printf("err: kafka:FetchMessages: %v\n", err.Error())
			}
			continue
		}
		log.Printf("fetch msgs' length: %v", len(msgs))

		success := false
		for i := 0; i < KafkaConsumerRetryTime; i++ {

			successKeys := make([]string, 0, len(msgs))
			failedKeys := make([]string, 0, len(msgs)) // 保存因 conver error 造成失败的 commentID
			tx := mysql.GetDB().Begin()                // 一批消息一个大的事务，整体成功或失败

			for _, msg := range msgs { // 并行消费 msg
				task := func(_msg kafka.Message) {
					uniqueKey, errorType, err1 := convertAndConsume(tx, consumer, _msg)
					if err1 != nil {
						if errorType == ErrTypeTransaction {
							err = errors.Wrap(err1, "kafka:CommentConsumer: convertAndConsume") // 保存事务中产生的错误
						} else {
							failedKeys = append(failedKeys, uniqueKey)
						}
					} else {
						successKeys = append(successKeys, uniqueKey)
					}
				}

				_msg := msg
				task(_msg)
			}

			// wg0.Wait() // 等待这一批数据处理完成

			if err != nil { // 说明在整个事务中，出现了错误，需要回滚事务，「不」向 kafka server 提交 offset
				// 打印日志
				log.Printf("kafka:CommentConsumer: convertAndConsume error: %v\n", err.Error())

				// 回滚事务
				tx.Rollback()

				// 重新消费这一批数据
				time.Sleep(time.Second)
				continue
			}

			tx.Commit()
			// logger.Debugf("事务提交")

			// 添加状态信息到 localcache 中
			for _, key := range successKeys {
				localcache.SetStatus(key, localcache.StatusSuccess)
			}

			for _, key := range failedKeys {
				localcache.SetStatus(key, localcache.StatusFailed)
			}

			success = true
			consumer.CommitMessages(context.TODO(), msgs[len(msgs)-1]) // 提交最后一个 offset（需保证该 consumer 对应的 group 的 consumer:partition = 1:1）
			break                                                      // 成功消费，退出 retry 循环
		}

		if !success { // 多次尝试后，仍失败
			logger.Warnf("kafka:basicConcurrentConsumerWork: Consume failed after %v retries, give up...", KafkaConsumerRetryTime)

			// 可以做一些其它操作，如添加到「死信队列」
			// 这里直接放弃请求
			// 后续可以添加其它策略
			consumer.CommitMessages(context.TODO(), msgs[len(msgs)-1]) // 提交最后一个 offset（需保证该 consumer 对应的 group 的 consumer:partition = 1:1）
		}

	}
}

// 返回 uniqueKey、error_type、error （可能是 convert，也可能是 consume）
func convertAndConsume(tx *gorm.DB, consumer *kafka.Reader, msg kafka.Message) (string, int, error) {
	var metadata Message
	err := json.Unmarshal(msg.Value, &metadata)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:convertAndConsume: Unmarshal(metadata)")
	}
	// tmp := metadata.Data.(map[string]any)
	// logger.Debugf("comment_id in Message.Data: %v", tmp["comment_id"])
	data, _ := json.Marshal(metadata.Data)
	var res Result

	switch metadata.Type {
	case TypeCommentCreate:
		return handleCommentCreate(tx, msg, data)

	case TypeCommentRemove:
		return handleCommentRemove(tx, data)

	case TypeLikeOrHateIncr:
		return handleLikeOrHateIncr(tx, data)

	case TypeLikeOrHateMappingCreate:
		return handleLikeOrHateMappingCreate(tx, data)

	case TypeLikeOrHateMappingRemove:
		return handleLikeOrHateMappingRemove(tx, data)
	}

	return res.UniqueKey, ErrTypeNoError, nil
}

// sync
func fetchMessages(ctx context.Context, reader *kafka.Reader, n int) ([]kafka.Message, error) {
	list := make([]kafka.Message, 0, n)
	msg, err := reader.FetchMessage(ctx) // 第一次使用 ctx
	if err != nil {
		return nil, errors.Wrap(err, "kafka:FetchMessages: FetchMessage")
	}
	list = append(list, msg)

	ctx1, cancel := context.WithTimeout(context.Background(), 8*time.Millisecond)
	defer cancel()

	// fetch 剩下的 n - 1 条消息
	for i := 0; i < n-1; i++ {
		msg, err = reader.FetchMessage(ctx1) // 后续调用设置独立超时时间
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) { // 如果是超时
				break
			}
			return nil, errors.Wrap(err, "kafka:FetchMessages: FetchMessage") // 其它错误
		}
		list = append(list, msg)
	}
	return list, nil
}

func handleCommentCreate(tx *gorm.DB, msg kafka.Message, data []byte) (string, int, error) {
	var params CommentCreate
	err := json.Unmarshal(data, &params)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleCommentCreate: Unmarshal(params)")
	}

	res := createComment(tx, msg, params)
	if res.Err != nil {
		return "", ErrTypeTransaction, errors.Wrap(res.Err, "kafka:handleCommentCreate: createComment")
	}

	return res.UniqueKey, ErrTypeNoError, nil
}

func handleCommentRemove(tx *gorm.DB, data []byte) (string, int, error) {
	var params CommentRemove
	err := json.Unmarshal(data, &params)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleCommentRemove: Unmarshal(params)")
	}
	res := removeComment(tx, params)
	if res.Err != nil {
		return "", ErrTypeTransaction, errors.Wrap(res.Err, "kafka:handleCommentRemove: removeComment")
	}

	return res.UniqueKey, ErrTypeNoError, nil
}

func handleLikeOrHateIncr(tx *gorm.DB, data []byte) (string, int, error) {
	var params LikeOrHateIncr
	err := json.Unmarshal(data, &params)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleLikeOrHateIncr: Unmarshal(params)")
	}
	res := incrCommentIndexCountField(tx, params.Field, params.CommentID, params.Offset)
	if res.Err != nil {
		return "", ErrTypeTransaction, errors.Wrap(res.Err, "kafka:handleLikeOrHateIncr: incrCommentIndexCountField")
	}

	return res.UniqueKey, ErrTypeNoError, nil
}

func handleLikeOrHateMappingCreate(tx *gorm.DB, data []byte) (string, int, error) {
	var params LikeOrHateMappingCreate
	err := json.Unmarshal(data, &params)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleLikeOrHateMappingCreate: Unmarshal(params)")
	}
	res := createCommentLikeOrHateUser(tx, params.CommentID, params.UserID, params.ObjID, params.ObjType, params.Like)
	if res.Err != nil {
		return "", ErrTypeTransaction, errors.Wrap(res.Err, "kafka:handleLikeOrHateMappingCreate: incrCommentIndexCountField")
	}

	return res.UniqueKey, ErrTypeNoError, nil
}

func handleLikeOrHateMappingRemove(tx *gorm.DB, data []byte) (string, int, error) {
	var params LikeOrHateMappingRemove
	err := json.Unmarshal(data, &params)
	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleLikeOrHateMappingRemoveByCommentIDs: Unmarshal(params)")
	}

	if err != nil {
		return "", ErrTypeConvert, errors.Wrap(err, "kafka:handleLikeOrHateMappingRemoveByCommentIDs: ConvertStringSliceToInt64Slice")
	}
	res := removeCommentUserLikeMappingByCommentIDs(tx, params.CommentID)
	if res.Err != nil {
		return "", ErrTypeTransaction, errors.Wrap(res.Err, "kafka:handleLikeOrHateMappingRemoveByCommentIDs: incrCommentIndexCountField")
	}

	return res.UniqueKey, ErrTypeNoError, nil
}
