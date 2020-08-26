package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient connect poll
type RedisClient struct {
	pool *redis.Client
}

// Lock redis
type Lock struct {
	key           string
	RequestID     string
	ttl           time.Duration
	timeout       time.Duration
	retryInterval time.Duration
}

var (
	once        sync.Once
	redisClient *RedisClient
)

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
		log.Println(pong, err)
		redisClient = &RedisClient{client}
	})
	return redisClient
}

//UserWalletHMSet ...
func (r *RedisClient) UserWalletHMSet(rd *RedisData) error {
	return r.pool.HMSet(ctx, rd.userKey, rd.hashMap).Err()
}

//UserWalletHGet ...
func (r *RedisClient) UserWalletHGet(rd *RedisData) int {
	val, err := r.pool.HGet(ctx, rd.userKey, redisHashWalletKey).Result()
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
func (r *RedisClient) Set(rd *RedisData) error {
	return r.pool.Set(ctx, rd.userKey, rd.amount, 0).Err()
}

// SetOrderlog write to log
func (r *RedisClient) SetOrderlog(rd *RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = r.pool.Set(ctx, rd.orderKey, j, 0).Err()
	if err != nil {
		panic(err)
	}
}

// SetRequestIDLog write to log, and check
func (r *RedisClient) SetRequestIDLog(rd *RedisData) (bool, error) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	ok, err := r.pool.SetNX(ctx, rd.requestID, j, rd.requestIDTTL*time.Second).Result()
	return ok, err

}

// Get value convert int
func (r *RedisClient) Get(rd *RedisData) int {
	val, err := r.pool.Get(ctx, rd.userKey).Result()
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
func (r *RedisClient) LPush(rd *RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = r.pool.LPush(ctx, rd.wallteOpKey, j).Err()
	if err != nil {
		fmt.Println(err)
	}
}

// TryLock redis
func (r *RedisClient) TryLock(l *Lock) error {
	getLock := make(chan bool, 1)
	timeout := time.After(l.timeout * time.Second)

	for {
		ok, err := r.pool.SetNX(ctx, l.key, l.RequestID, l.ttl*time.Second).Result()
		if err != nil {
			return err
		}
		getLock <- ok

		select {
		case <-timeout:
			return errors.New(fmt.Sprintln(l.key, l.RequestID, "Lock wait to timeout."))
		case v := <-getLock:
			if !v {
				time.Sleep(l.retryInterval * time.Second)
				break
			}
			return nil
		}
	}
}

// UnLock redis lock
func (r *RedisClient) UnLock(l *Lock) error {
	return r.pool.Del(ctx, l.key).Err()
}

// Close redis close
func (r *RedisClient) Close() error {
	return r.pool.Close()
}
