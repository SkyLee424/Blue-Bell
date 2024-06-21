package kafka

import (
	"bluebell/dao/localcache"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
)

const (
	TypeCommentCreate = iota + 1
	TypeCommentRemove
	TypeCommentRemoveByObjID
	TypeLikeOrHateIncr
	TypeLikeOrHateMappingCreate
	TypeLikeOrHateMappingRemove
	TypeEmailSendVerificationCode
)

const (
	ErrTypeNoError     = iota + 1
	ErrTypeConvert     // 本不应该产生，是系统内部的错误
	ErrTypeTransaction // 事务执行时产生的错误
)

const (
	TopicComment = "topic-comment"
	TopicLike    = "topic-like"
	TopicEmail   = "topic-email"
)

const (
	GroupComment = "group-comment"
	GroupLike    = "group-like"
	GroupEmail   = "group-email"
)

var addr []string

var (
	PartitionNumOfComment = 6
	PartitionNumOfLike    = 6
	PartitionNumOfEmail   = 2
)

var (
	ReplicationFactorOfComment = 1
	ReplicationFactorOfLike    = 1
	ReplicationFactorOfEmail   = 1
)

var (
	KafkaProducerRetryTime = 5 // 发送失败，重试次数
	KafkaConsumerRetryTime = 5 // 消费失败，重试次数
)

type Message struct {
	Type int8 `json:"type"`
	Data any  `json:"data"`
}

type Result struct {
	UniqueKey string // 一条消息消费结束后，可以代表的唯一标识
	Err       error  // 消费产生的错误
}

var commentWriter *kafka.Writer
var likeWriter *kafka.Writer
var emailWriter *kafka.Writer

var notifyList []chan int

var wg sync.WaitGroup

func InitKafka() {
	initConfig()

	// 初始化 producer
	commentWriter = &kafka.Writer{
		Addr:         kafka.TCP(addr...),
		Balancer:     &kafka.Hash{},    // 轮询
		RequiredAcks: kafka.RequireAll, // all
	}

	likeWriter = &kafka.Writer{
		Addr:     kafka.TCP(addr...),
		Balancer: &kafka.Hash{}, // 哈希，保证相同的 comment 在同一个 partition
	}

	emailWriter = &kafka.Writer{
		Addr:     kafka.TCP(addr...),
		Balancer: &kafka.RoundRobin{},
	}

	// 初始化通知列表
	notifyList = make([]chan int, 0, PartitionNumOfComment+PartitionNumOfLike+PartitionNumOfEmail)

	// 创建主题
	createTopic(TopicComment, PartitionNumOfComment, ReplicationFactorOfComment)
	createTopic(TopicLike, PartitionNumOfLike, ReplicationFactorOfLike)
	createTopic(TopicEmail, PartitionNumOfEmail, ReplicationFactorOfEmail)

	// 初始化 consumer
	initConsumer(PartitionNumOfComment, TopicComment, GroupComment)
	initConsumer(PartitionNumOfLike, TopicLike, GroupLike)
	initConsumer(PartitionNumOfEmail, TopicEmail, GroupEmail)
}

func Wait() {
	// 通知消费者退出
	for i := 0; i < len(notifyList); i++ {
		notifyList[i] <- 1
	}

	wg.Wait()
}

// 轮询消息是否消费，超时返回 false，错误返回 error
func CheckIfConsumed(uniqueKey string, retry, interval int) (consumed bool, err error) {
	consumed = false
	for i := 0; i < retry; i++ {
		time.Sleep(time.Millisecond * time.Duration(interval))
		status, ok := localcache.GetStatus(uniqueKey)
		if !ok {
			continue
		}

		consumed = true
		if status == localcache.StatusFailed {
			err = errors.New("message has been consumed but failed")
		}

		localcache.RemoveStatus(uniqueKey)
		break
	}
	return
}

func initConfig() {
	addr = viper.GetStringSlice("kafka.addr")

	PartitionNumOfComment = viper.GetInt("kafka.partition.comment")
	PartitionNumOfLike = viper.GetInt("kafka.partition.like")
	PartitionNumOfLike = viper.GetInt("kafka.partition.email")

	ReplicationFactorOfComment = viper.GetInt("kafka.replication_factor.comment")
	ReplicationFactorOfLike = viper.GetInt("kafka.replication_factor.like")
	ReplicationFactorOfLike = viper.GetInt("kafka.replication_factor.email")

	KafkaProducerRetryTime = viper.GetInt("kafka.retry.producer")
	KafkaConsumerRetryTime = viper.GetInt("kafka.retry.consumer")

}

func createTopic(topicName string, partitionNum, replicationFactor int) {
	// 连接至任意kafka节点
	if len(addr) == 0 {
		panic("kafka address length should not be zero")
	}
	conn, err := kafka.Dial("tcp", addr[0])
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	// 获取当前控制节点信息
	controller, err := conn.Controller()
	if err != nil {
		panic(err.Error())
	}
	var controllerConn *kafka.Conn
	// 连接至leader节点
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		panic(err.Error())
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topicName,
			NumPartitions:     partitionNum,
			ReplicationFactor: replicationFactor,
		},
	}

	// 创建topic
	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		panic(err.Error())
	}
}

// 创建消费者
func initConsumer(partitionNum int, topic, group string) {
	// 每个 partition 对应一个 consumer
	wg.Add(partitionNum)
	for i := 0; i < partitionNum; i++ {
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers: addr,
			Topic:   topic,
			GroupID: group,
		})

		ch := make(chan int, 1)
		notifyList = append(notifyList, ch)
		go basicSerialConsumerWork(ch, r)
	}
}
