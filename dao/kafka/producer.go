package kafka

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
)

func writeMessage(writer *kafka.Writer, topic, key string, _type int8, content any) (err error) {
	metadata := Message{
		Type: _type,
		Data: content,
	}
	val, _ := json.Marshal(metadata)
	var tmp Message
	json.Unmarshal(val, &tmp)
	// logger.Debugf("key: %v, Message.Data(unmarshal): %d", key, tmp.Data.(map[string]any)["comment_id"])

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: val,
	}

	// 投递消息到 kafka
	for i := 0; i < KafkaProducerRetryTime; i++ {
		err = writer.WriteMessages(context.TODO(), msg)
		if err == nil {
			return
		}
	}
	return errors.Wrap(err, "kafka-producer:writeMessage: WriteMessages")
}
