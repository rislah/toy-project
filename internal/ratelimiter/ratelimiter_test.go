package ratelimiter_test

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cep21/circuit/v3"
	"github.com/rislah/fakes/internal/ratelimiter"
	"github.com/rislah/fakes/internal/redis"
	"github.com/stretchr/testify/assert"
)

type rateLimiterTestCase struct {
	client      redis.Client
	ratelimiter *ratelimiter.Ratelimiter

	field          ratelimiter.Field
	limitPerMinute int
	windowInterval time.Duration
	bucketInterval time.Duration
}

func TestRateLimiter(t *testing.T) {
	tests := []struct {
		scenario       string
		field          ratelimiter.Field
		limitPerMinute int
		name           string
		writeHeaders   bool
		windowInterval time.Duration
		bucketInterval time.Duration
		test           func(ctx context.Context, rateLimiterTestCase rateLimiterTestCase)
	}{
		{
			scenario:       "should ratelimit headers",
			field:          ratelimiter.Field{Scope: "test", Identifier: "127.0.0.1"},
			limitPerMinute: 2,
			windowInterval: 1 * time.Minute,
			name:           "test",
			writeHeaders:   true,
			bucketInterval: 5 * time.Second,
			test: func(ctx context.Context, rateLimiterTestCase rateLimiterTestCase) {
				rw := httptest.NewRecorder()
				throttled, err := rateLimiterTestCase.ratelimiter.ShouldThrottle(ctx, rw, rateLimiterTestCase.field)
				assert.NoError(t, err)
				assert.False(t, throttled)

				ratelimitHeader := rw.Header().Get("RateLimit-Limit")
				ratelimitResetHeader := rw.Header().Get("RateLimit-Reset")
				ratelimitRemainingHeader := rw.Header().Get("RateLimit-Remaining")

				assert.Equal(t, strconv.Itoa(rateLimiterTestCase.limitPerMinute), ratelimitHeader)
				assert.NotEmpty(t, ratelimitResetHeader)
				assert.Equal(t, "2", ratelimitRemainingHeader)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			srv, err := miniredis.Run()
			assert.NoError(t, err)

			client, err := redis.NewClient(srv.Addr(), &circuit.Circuit{}, nil)
			assert.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer client.FlushAll()
			defer cancel()
			defer srv.Close()

			opts := &ratelimiter.Options{
				Datastore:      ratelimiter.NewRedisDatastore(client),
				Name:           test.name,
				LimitPerMinute: test.limitPerMinute,
				WindowInterval: test.windowInterval,
				BucketInterval: test.bucketInterval,
				WriteHeaders:   test.writeHeaders,
			}

			rl := ratelimiter.NewRateLimiter(opts)
			test.test(ctx, rateLimiterTestCase{
				client:         client,
				ratelimiter:    rl,
				field:          test.field,
				limitPerMinute: test.limitPerMinute,
				windowInterval: test.windowInterval,
				bucketInterval: test.bucketInterval,
			})
		})
	}
}
