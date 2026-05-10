package cache

import (
	"context"
	"github.com/go-redis/redis/v9"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"time"

	"github.com/pkg/errors"
)

const (
	FundTransferTokenKey = "OAUTHTOKEN:FUNDTRANSFER"
	OAuthCountKey        = "OAUTHTOKEN:COUNT:%s"
	PaymentKey           = "PAYMENT:%s"
)

var mode string

const CHASP = "CH:ASP"

type redisClient struct {
	ClusterClient *redis.ClusterClient
	Client        *redis.Client
}

func (r redisClient) CMD() redis.Cmdable {
	if mode == "cluster" {
		return r.ClusterClient
	}
	return r.Client
}

func (r *redisClient) Close() error {
	if mode == "cluster" {
		return r.ClusterClient.Close()
	}
	return r.Client.Close()
}

func (r *redisClient) UniversalClient() redis.UniversalClient {
	if mode == "cluster" {
		return r.ClusterClient
	}
	return r.Client
}

func Initialize(ctx context.Context, config config.RedisConfig) (*redisClient, error) {

	var client = &redisClient{}
	mode = config.Mode

	cliCh := make(chan string)
	errCh := make(chan error)

	if config.Mode == "normal" {
		client.Client = redis.NewClient(&redis.Options{
			Addr:         config.Host + ":" + config.Port,
			Password:     config.Password,
			DB:           config.DB,
			PoolTimeout:  time.Second * 10,
			DialTimeout:  time.Second * 10,
			WriteTimeout: time.Second * 5,
			ReadTimeout:  -1,
		})
		go func() {
			cli, err := client.Client.Ping(ctx).Result()
			if err != nil {
				errCh <- err
			}
			if cli == "PONG" {
				cliCh <- cli
			}
		}()
	} else if config.Mode == "cluster" {
		client.ClusterClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Cluster.Addr,
			PoolTimeout:  time.Second * 10,
			DialTimeout:  time.Second * 10,
			WriteTimeout: time.Second * 10,
			ReadTimeout:  -1,
		})
		go func() {
			cli, err := client.ClusterClient.Ping(ctx).Result()
			if err != nil {
				errCh <- err
			}
			if cli == "PONG" {
				cliCh <- cli
			}
		}()
	}

	select {
	case <-cliCh:
		return client, nil
	case errMsg := <-errCh:
		return nil, errors.New("Cannot connect to redis : " + errMsg.Error())
	case <-ctx.Done():
		return nil, errors.New("connect to redis timeout")
	}

}

type HGetAllRedisFunc func(ctx context.Context, key string) (map[string]string, error)

func HGetAllRedis(cmd redis.Cmdable) HGetAllRedisFunc {
	return func(ctx context.Context, key string) (map[string]string, error) {
		return cmd.HGetAll(ctx, key).Result()
	}
}

type SetRedisNXFunc func(ctx context.Context, key string, value interface{}, expiration time.Duration) error

func SetRedisNX(cmd redis.Cmdable) SetRedisNXFunc {
	return func(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
		return cmd.SetNX(ctx, key, value, expiration).Err()
	}
}

type GetRedisFunc func(ctx context.Context, key string) (string, error)

func GetRedis(cmd redis.Cmdable) GetRedisFunc {
	return func(ctx context.Context, key string) (string, error) {
		return cmd.Get(ctx, key).Result()
	}
}

type DelRedisFunc func(ctx context.Context, key string) error

func DeleteRedis(cmd redis.Cmdable) DelRedisFunc {
	return func(ctx context.Context, key string) error {
		return cmd.Del(ctx, key).Err()
	}
}

type SetRedisFunc func(ctx context.Context, key string, value interface{}, expiration time.Duration) error

func SetRedis(cmd redis.Cmdable) SetRedisFunc {
	return func(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
		return cmd.Set(ctx, key, value, expiration).Err()
	}
}

type InCrRedisFunc func(ctx context.Context, key string) (int64, error)

func InCrRedis(cmd redis.Cmdable) InCrRedisFunc {
	return func(ctx context.Context, key string) (int64, error) {
		return cmd.Incr(ctx, key).Result()
	}
}

type SetExpireFunc func(ctx context.Context, key string, exp time.Duration) error

func SetExpire(cmd redis.Cmdable) SetExpireFunc {
	return func(ctx context.Context, key string, exp time.Duration) error {
		return cmd.Expire(ctx, key, exp).Err()
	}
}

type PingFunc func(ctx context.Context) error

func Ping(cmd redis.Cmdable) PingFunc {
	return func(ctx context.Context) error {
		return cmd.Ping(ctx).Err()
	}
}
