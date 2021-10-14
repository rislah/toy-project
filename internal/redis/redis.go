package redis

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rislah/fakes/internal/errors"
	"github.com/rislah/fakes/internal/logger"
	"github.com/rislah/fakes/internal/metrics"

	"github.com/cep21/circuit"
	"github.com/go-redis/redis/v8"
)

var (
	sharedRedisClient *redis.Client
)

type Client interface {
	Get(key string) (string, error)
	GetBool(key string) (bool, error)
	GetInt64(key string) (int64, error)
	Set(key string, val interface{}, ttl time.Duration) error
	Del(key string) error
	Exists(key string) bool
	Eval(script string, keys, args []string) (interface{}, error)
	TTL(key string) (time.Duration, error)
	Ping() error
	Close() error
}

type clientImpl struct {
	client *redis.Client
	cb     *circuit.Circuit
	logger *logger.Logger
}

func NewClient(uri string, cm *circuit.Manager, mtr *metrics.Metrics, lg *logger.Logger) (*clientImpl, error) {
	client, err := newClientPkg(uri)
	if err != nil {
		return nil, err
	}

	c := cm.MustCreateCircuit(
		"redis",
		circuit.Config{
			Execution: circuit.ExecutionConfig{
				Timeout: 500 * time.Millisecond,
			},
		},
	)

	return &clientImpl{
		client: client,
		cb:     c,
		logger: lg,
	}, nil
}

func newClientPkg(uri string) (*redis.Client, error) {
	if sharedRedisClient != nil {
		return sharedRedisClient, nil
	}

	opts := &redis.Options{
		Network:            "tcp",
		Addr:               uri,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        500 * time.Millisecond,
		WriteTimeout:       500 * time.Second,
		IdleTimeout:        20 * time.Second,
		IdleCheckFrequency: 15 * time.Second,
	}

	opts.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return net.DialTimeout(opts.Network, uri, opts.DialTimeout)
	}

	sharedRedisClient = redis.NewClient(opts)

	if err := sharedRedisClient.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return sharedRedisClient, nil
}

func (c *clientImpl) Get(key string) (string, error) {
	var result string
	var outErr error
	err := c.cb.Go(context.Background(), func(ctx context.Context) error {
		var redisErr error
		if result, redisErr = c.client.Get(ctx, key).Result(); redisErr != nil {
			if redisErr == redis.Nil {
				outErr = redisErr
				return nil
			}
			return redisErr
		}
		return nil
	}, nil)
	if err != nil {
		c.logger.Warn("get()", err, nil)
		return "", err
	}
	return result, outErr
}

func (c *clientImpl) GetBool(key string) (bool, error) {
	result, err := c.Get(key)
	if err != nil {
		return false, err
	}

	switch result {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("GetBool: could not parse key=%s value=%s", key, result)
	}
}

func (c *clientImpl) GetInt64(key string) (int64, error) {
	var result int64
	err := c.cb.Go(context.Background(), func(ctx context.Context) error {
		var err error
		if result, err = c.client.Get(ctx, key).Int64(); err != nil {
			if err == redis.Nil {
				return &circuit.SimpleBadRequest{
					Err: err,
				}
			}

			return err
		}

		return nil
	}, nil)

	if err != nil {
		if !errors.IsWrappedRedisNilError(err) {
			c.logger.Warn("GetInt64", err, nil)
		}

		return -1, err
	}

	return result, nil
}

func (c *clientImpl) Set(key string, val interface{}, ttl time.Duration) error {
	err := c.cb.Go(context.Background(), func(ctx context.Context) error {
		if err := c.client.Set(ctx, key, val, ttl).Err(); err != nil {
			c.logger.Warn("Set", err, nil)
			return &circuit.SimpleBadRequest{
				Err: err,
			}
		}

		return nil
	}, nil)

	if err != nil {
		return err
	}

	return nil
}

func (c *clientImpl) Del(key string) error {
	err := c.cb.Go(context.Background(), func(ctx context.Context) error {
		if err := c.client.Del(ctx, key).Err(); err != nil {
			c.logger.Warn("Del()", err, nil)
			return &circuit.SimpleBadRequest{
				Err: err,
			}
		}

		return nil
	}, nil)

	if err != nil {
		return err
	}

	return nil
}

func (c *clientImpl) Exists(key string) bool {
	var result bool
	_ = c.cb.Go(context.Background(), func(ctx context.Context) error {
		result = c.client.Exists(ctx, key).Val() > 1
		return nil
	}, nil)

	return result
}

func (c *clientImpl) Eval(script string, keys, args []string) (interface{}, error) {
	var result interface{}
	err := c.cb.Go(context.Background(), func(ctx context.Context) error {
		var err error
		if result, err = c.client.Eval(ctx, script, keys, args).Result(); err != nil {
			c.logger.Warn("Eval()", err, nil)
			return &circuit.SimpleBadRequest{
				Err: err,
			}
		}

		return nil
	}, nil)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *clientImpl) Ping() error {
	return c.client.Ping(context.Background()).Err()
}

func (c *clientImpl) Close() error {
	return c.client.Close()
}

func (c *clientImpl) TTL(key string) (time.Duration, error) {
	result, err := c.client.TTL(context.Background(), key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	return result, nil
}
