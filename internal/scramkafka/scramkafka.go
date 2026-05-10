package scramkafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"github.com/xdg-go/scram"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"

	"go.uber.org/zap"
	"hash"
)

var KafkaSHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }

var KafkaSHA512 scram.HashGeneratorFcn = func() hash.Hash { return sha512.New() }

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}

func NewConsumerClient(cfg config.KafkaConfig) (sarama.ConsumerGroup, error) {

	if len(cfg.Group) == 0 {
		zap.L().Fatal("no Kafka consumer group defined, please set the -group flag")
	}

	version, err := sarama.ParseKafkaVersion(cfg.Version)
	if err != nil {
		return nil, errors.Wrap(err, "parsing kafka version")
	}

	config := sarama.NewConfig()
	config.Version = version
	config.ClientID = "1"
	config.Consumer.Fetch.Default = 1
	config.Consumer.Offsets.AutoCommit.Enable = true

	if cfg.Oldest {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	if cfg.SSAL {
		config.Net.TLS.Enable = cfg.SSAL
		config.Net.TLS.Config = createKafkaTLSConfiguration(cfg.Certs)
		config.Net.SASL.Handshake = cfg.SSAL
		config.Net.SASL.User = cfg.Username
		config.Net.SASL.Password = cfg.Password
		config.Net.SASL.Enable = cfg.SSAL
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: KafkaSHA512} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	}

	switch cfg.Strategy {
	case "roundrobin":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	case "rang":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	default:
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	}

	return sarama.NewConsumerGroup(cfg.Brokers, cfg.Group, config)

}

func NewAsyncProducer(cfg config.KafkaConfig) (sarama.AsyncProducer, error) {
	config := sarama.NewConfig()

	version, err := sarama.ParseKafkaVersion(cfg.Version)

	if err != nil {
		return nil, errors.Wrap(err, "parsing kafka version")
	}

	config.Version = version
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 10

	if cfg.SSAL {
		config.Net.TLS.Enable = cfg.TLS
		config.Net.TLS.Config = createKafkaTLSConfiguration(cfg.Certs)
		config.Net.SASL.Handshake = cfg.SSAL
		config.Net.SASL.User = cfg.Username
		config.Net.SASL.Password = cfg.Password
		config.Net.SASL.Enable = cfg.SSAL
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: KafkaSHA512} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	}

	switch cfg.Strategy {
	case "roundrobin":
		config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	case "rang":
		config.Producer.Partitioner = sarama.NewHashPartitioner
	default:
		config.Producer.Partitioner = sarama.NewRandomPartitioner
	}

	return sarama.NewAsyncProducer(cfg.Brokers, config)
}

func NewSyncProducer(cfg config.KafkaConfig) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()

	version, err := sarama.ParseKafkaVersion(cfg.Version)

	if err != nil {
		return nil, errors.Wrap(err, "parsing kafka version")
	}

	config.Version = version
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 10

	if cfg.SSAL {
		config.Net.TLS.Enable = cfg.TLS
		config.Net.TLS.Config = createKafkaTLSConfiguration(cfg.Certs)
		config.Net.SASL.Handshake = cfg.SSAL
		config.Net.SASL.User = cfg.Username
		config.Net.SASL.Password = cfg.Password
		config.Net.SASL.Enable = cfg.SSAL
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: KafkaSHA512} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	}

	switch cfg.Strategy {
	case "roundrobin":
		config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	case "rang":
		config.Producer.Partitioner = sarama.NewHashPartitioner
	default:
		config.Producer.Partitioner = sarama.NewRandomPartitioner
	}

	return sarama.NewSyncProducer(cfg.Brokers, config)
}

func createKafkaTLSConfiguration(certFile string) (t *tls.Config) {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(certFile))

	return &tls.Config{
		InsecureSkipVerify: true,
		RootCAs:            caCertPool,
	}
}
