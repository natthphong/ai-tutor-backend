package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

type SendMessageAsyncFunc func(event interface{})

type SendMessageSyncFunc func(log *zap.Logger, event interface{}) error
type SendMessageSyncWithTopicFunc func(log *zap.Logger, event interface{}, topic string) error

func NewSendMessageSyncWithTopic(producer sarama.SyncProducer) SendMessageSyncWithTopicFunc {

	return func(log *zap.Logger, event interface{}, topic string) error {
		value, err := json.Marshal(event)

		if err != nil {
			return err
		}

		message := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(value)}
		partition, offset, err := producer.SendMessage(message)

		if err != nil {
			return errors.New(fmt.Sprintf("topic: %s partition: %v, offset: %v, error: %v", topic, partition, offset, err))
		}

		log.Info(fmt.Sprintf("SendMessage Success with topic: %s, Partition: %v, Offset: %v", topic, partition, offset), zap.Reflect("Message", event))

		return nil
	}
}
func NewAsyncSendMessage(producer sarama.AsyncProducer, topic string) SendMessageAsyncFunc {
	return func(event interface{}) {
		value, err := json.Marshal(event)
		if err != nil {
			return
		}
		message := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(value)}
		producer.Input() <- message
	}
}

func NewSyncSendMessage(producer sarama.SyncProducer, topic string) SendMessageSyncFunc {
	return func(log *zap.Logger, event interface{}) error {
		value, err := json.Marshal(event)

		if err != nil {
			return err
		}

		message := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(value)}
		partition, offset, err := producer.SendMessage(message)

		if err != nil {
			return errors.New(fmt.Sprintf("topic: %s partition: %v, offset: %v, error: %v", topic, partition, offset, err))
		}

		log.Info(fmt.Sprintf("SendMessage Success with topic: %s, Partition: %v, Offset: %v", topic, partition, offset), zap.Reflect("Message", event))

		return nil
	}
}
