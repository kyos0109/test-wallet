package kredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/kyos0109/test-wallet/modules"
)

const (
	RedisDelimiter         = ":"
	RedisOrderPerfix       = "Orders"
	RedisUserPerfix        = "Users"
	RedisLockOpsPerfix     = "LockOps"
	RediswallteOpsPerfix   = "walletOps"
	RedisHashWalletKey     = "wallet"
	RedisHashlastChangeKey = "lastChange"
	RedisHashlastGameKey   = "lastGameID"
	RedisLockTTL           = 3600
	RedisLockTimeout       = 30
	RedisLockRetryInterval = 1

	PostDeductCmd = "deduct"
	PostStoreCmd  = "store"
)

// RedisClient connect poll
type RedisClient struct {
	pool *redis.Client
}

var (
	once        sync.Once
	redisClient *RedisClient
	ctx         context.Context
)

// InitWithCtx ...
func InitWithCtx(pctx *context.Context) {
	ctx = *pctx
}

// GetRedisClientInstance Singleton
func GetRedisClientInstance() *RedisClient {
	once.Do(func() {
		client := redis.NewClient(&redis.Options{
			Addr:         "127.0.0.1:6379",
			Password:     "",
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     200,
			PoolTimeout:  0,
			IdleTimeout:  0,
			DB:           0,
		})

		pong, err := client.Ping(ctx).Result()
		if err != nil {
			log.Fatalln(pong, err)
		}
		log.Println(pong)

		redisClient = &RedisClient{client}
	})
	return redisClient
}

//UserWalletHMSet ...
func (r *RedisClient) UserWalletHMSet(rd *modules.RedisData) error {
	return r.pool.HMSet(ctx, rd.UserKey, rd.HashMap).Err()
}

//UserWalletHGet ...
func (r *RedisClient) UserWalletHGet(rd *modules.RedisData) int {
	val, err := r.pool.HGet(ctx, rd.UserKey, RedisHashWalletKey).Result()
	if err == redis.Nil {
		return -1
	}
	if err != nil {
		panic(err)
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}

	return i
}

// Set key and value
func (r *RedisClient) Set(rd *modules.RedisData) error {
	return r.pool.Set(ctx, rd.UserKey, rd.Amount, 0).Err()
}

// SetOrderlog write to log
func (r *RedisClient) SetOrderlog(rd *modules.RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = r.pool.Set(ctx, rd.OrderKey, j, 0).Err()
	if err != nil {
		panic(err)
	}
}

// SetRequestIDLog write to log, and check
func (r *RedisClient) SetRequestIDLog(rd *modules.RedisData) (bool, error) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	ok, err := r.pool.SetNX(ctx, rd.RequestID, j, rd.RequestIDTTL*time.Second).Result()
	return ok, err

}

// Get value convert int
func (r *RedisClient) Get(rd *modules.RedisData) int {
	val, err := r.pool.Get(ctx, rd.UserKey).Result()
	if err == redis.Nil {
		return -1
	}
	if err != nil {
		panic(err)
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}

	return i
}

// LPush write list
func (r *RedisClient) LPush(rd *modules.RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = r.pool.LPush(ctx, rd.WallteOpKey, j).Err()
	if err != nil {
		fmt.Println(err)
	}
}

// TryLock redis
func (r *RedisClient) TryLock(l *modules.Lock) error {
	getLock := make(chan bool, 1)
	timeout := time.After(l.Timeout * time.Second)

	for {
		ok, err := r.pool.SetNX(ctx, l.Key, l.RequestID, l.TTL*time.Second).Result()
		if err != nil {
			return err
		}
		getLock <- ok

		select {
		case <-timeout:
			return errors.New(fmt.Sprintln(l.Key, l.RequestID, "Lock wait to timeout."))
		case v := <-getLock:
			if !v {
				time.Sleep(l.RetryInterval * time.Second)
				break
			}
			return nil
		}
	}
}

// UnLock redis lock
func (r *RedisClient) UnLock(l *modules.Lock) error {
	return r.pool.Del(ctx, l.Key).Err()
}

// Close redis close
func (r *RedisClient) Close() error {
	return r.pool.Close()
}
