package kredis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/modules"
)

const (
	Nil             = redis.Nil
	noneDataTimeout = 60
)

// RedisClient connect pool
type RedisClient struct {
	pool *redis.Client
}

// RedisClusterClient connect pool
type RedisClusterClient struct {
	pool *redis.ClusterClient
}

var (
	once               sync.Once
	redisClient        *RedisClient
	redisClusterClient *RedisClusterClient
	ctx                context.Context
	host               string
	password           string
	port               string
)

// InitWithCtx ...
func InitWithCtx(pctx *context.Context, c *config.RedisConfig) context.Context {
	ctx = *pctx

	host = c.Host
	port = strconv.Itoa(c.Port)
	password = c.Password

	return context.WithValue(*pctx, RedisClient{}, GetRedisClientInstance())
}

// GetRedisClientInstance Singleton
func GetRedisClientInstance() *RedisClient {
	once.Do(func() {
		client := redis.NewClient(&redis.Options{
			Addr:         host + ":" + port,
			Password:     password,
			MaxRetries:   config.Redis.ConnectReTry,
			DialTimeout:  5 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     config.Redis.ConnectPool,
			PoolTimeout:  0,
			IdleTimeout:  0,
			DB:           0,
		})

		pong, err := client.Ping(ctx).Result()
		if err != nil {
			log.Println(pong, err)
		}
		log.Println("Test Redis Ping: ", pong)

		redisClient = &RedisClient{client}
	})
	return redisClient
}

// GetRedisClusterClientInstance Singleton
func GetRedisClusterClientInstance() *RedisClusterClient {
	once.Do(func() {
		client := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        []string{host + ":" + port},
			Password:     password,
			MaxRetries:   config.Redis.ConnectReTry,
			DialTimeout:  5 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     config.Redis.ConnectPool,
			PoolTimeout:  0,
			IdleTimeout:  0,
		})

		pong, err := client.Ping(ctx).Result()
		if err != nil {
			log.Println(pong, err)
		}
		log.Println(pong)

		redisClusterClient = &RedisClusterClient{client}
	})
	return redisClusterClient
}

//UserWalletHMSet ...
func (r *RedisClient) UserWalletHMSet(rd *modules.RedisData) error {
	return r.pool.HMSet(ctx, rd.UserKey, rd.HashMap).Err()
}

//UserWalletHGet ...
func (r *RedisClient) UserWalletHGet(rd *modules.RedisData) int {
	val, err := r.pool.HGet(ctx, rd.UserKey, config.RedisHashWalletKey).Result()
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

//UserWalletIDHGet ...
func (r *RedisClient) UserWalletIDHGet(rd *modules.RedisData) int {
	val, err := r.pool.HGet(ctx, rd.UserKey, config.RedisHashWalletIDKey).Result()
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

//UserStatusHGet ...
func (r *RedisClient) UserStatusHGet(rd *modules.RedisData) bool {
	val, err := r.pool.HGet(ctx, rd.UserKey, config.RedisHashlastStatusKey).Result()
	if err == redis.Nil {
		return false
	}
	if err != nil {
		log.Println(err)
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		log.Println(err)
	}

	return b
}

// SetOrderlog write to log
func (r *RedisClient) SetOrderlog(rd *modules.RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		log.Println(err)
		return
	}

	err = r.pool.Set(ctx, rd.OrderKey, j, config.Redis.OrdersDataTTL).Err()
	if err != nil {
		log.Println(err)
		return
	}
}

// SetRequestIDLog write to log, and check
func (r *RedisClient) SetRequestIDLog(rd *modules.RedisData) (bool, error) {
	j, err := json.Marshal(&rd)
	if err != nil {
		return false, err
	}

	ok, err := r.pool.SetNX(ctx, rd.RequestID, j, config.Redis.RequestIDTTL).Result()
	return ok, err

}

// LPush write list
func (r *RedisClient) LPush(rd *modules.RedisData) {
	j, err := json.Marshal(&rd)
	if err != nil {
		log.Println(err)
		return
	}

	err = r.pool.LPush(ctx, rd.WallteOpKey, j).Err()
	if err != nil {
		log.Println(err)
		return
	}
}

// LPushJob write list
func (r *RedisClient) LPushJob(rd *modules.RedisData) error {
	j, err := json.Marshal(&rd)
	if err != nil {
		return err
	}

	err = r.pool.LPush(ctx, config.RedisJobsQueue, j).Err()
	if err != nil {
		return err
	}
	return nil
}

// RPopLPushJobResStruct ...
func (r *RedisClient) RPopLPushJobResStruct() (*modules.RedisData, error) {
	rd := &modules.RedisData{}

	res, err := r.pool.RPopLPush(ctx, config.RedisJobsQueue, config.RedisJobProcessing).Result()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(res), &rd); err != nil {
		return nil, err
	}
	return rd, nil
}

// BRPopLPushJobResStruct ...
func (r *RedisClient) BRPopLPushJobResStruct() (*modules.RedisData, error) {
	rd := &modules.RedisData{}

	res, err := r.pool.BRPopLPush(ctx, config.RedisJobsQueue, config.RedisJobProcessing, time.Second*noneDataTimeout).Result()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(res), &rd); err != nil {
		return nil, err
	}
	return rd, nil
}

// LRemProcessing ...
func (r *RedisClient) LRemProcessing(rd *modules.RedisData, count int64) error {
	j, err := json.Marshal(&rd)
	if err != nil {
		return err
	}

	err = r.pool.LRem(ctx, config.RedisJobProcessing, count, j).Err()
	if err != nil {
		return err
	}
	return nil
}

// TryLock redis
func (r *RedisClient) TryLock(l *modules.Lock) error {
	getLock := make(chan bool, 1)
	timeout := time.After(config.Redis.LockTimeout)

	for {
		ok, err := r.pool.SetNX(ctx, l.Key, l.RequestID, config.Redis.LockTTL).Result()
		if err != nil {
			return err
		}
		getLock <- ok

		select {
		case <-timeout:
			if err := r.PushFailRequestKey(l.RequestID); err != nil {
				return err
			}
			return fmt.Errorf("Key: %s, RequestID: %s Lock wait to timeout", l.Key, l.RequestID)
		case v := <-getLock:
			if !v {
				time.Sleep(config.Redis.LockRetryInterval)
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

// Publish ...
func (r *RedisClient) Publish(channelName string, rd *modules.RedisData) *redis.IntCmd {
	return r.pool.Publish(ctx, channelName, rd)
}

// Conn ...
func (r *RedisClient) Conn() *redis.Client {
	return r.pool
}

// LLen ...
func (r *RedisClient) LLen(key string) (int64, error) {
	i, err := r.pool.LLen(ctx, key).Result()
	if err != nil {
		return -1, err
	}
	return i, nil
}

// Get ...
func (r *RedisClient) Get(key string) (string, error) {
	s, err := r.pool.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s, nil
}

//HMSet ...
func (r *RedisClient) HMSet(key string, hashMap map[string]interface{}) error {
	return r.pool.HMSet(ctx, key, hashMap).Err()
}

//HGet ...
func (r *RedisClient) HGet(key string, field string) string {
	s, err := r.pool.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return ""
	}
	if err != nil {
		log.Println(err)
	}

	return s
}

//Expire ...
func (r *RedisClient) Expire(key string, ttl time.Duration) bool {
	b, err := r.pool.Expire(ctx, key, ttl).Result()
	if err == redis.Nil {
		return false
	}
	if err != nil {
		log.Println(err)
	}

	return b
}

//ExpireAt ...
func (r *RedisClient) ExpireAt(key string, timestamp time.Time) bool {
	b, err := r.pool.ExpireAt(ctx, key, timestamp).Result()
	if err == redis.Nil {
		return false
	}
	if err != nil {
		log.Println(err)
	}

	return b
}

//PushFailRequestKey ...
func (r *RedisClient) PushFailRequestKey(key string) error {
	v, err := r.Get(key)
	if err == redis.Nil {
		return fmt.Errorf("Set Timeout Request: %s Not Found", key)
	}
	if err != nil {
		return err
	}

	err = r.pool.LPush(ctx, config.RedisFailedRequestKey, v).Err()
	if err != nil {
		return err
	}

	return nil
}

// func createSyncDBCacheScript() *redis.Script {
// 	return redis.NewScript(`
// 		local Expire = ARGV[1]
// 		local res = redis.call("HMSET", KEYS[1], unpack(ARGV))
// 		local res = redis.call("EXPIRE", KEYS[1], 10)

// 		return Expire
// 	`)
// }

//SyncDBCache ...
// func (r *RedisClient) SyncDBCache(key string, hashMap map[string]interface{}) (bool, error) {

// 	script := createSyncDBCacheScript()

// 	sha, err := script.Load(ctx, r.pool).Result()
// 	if err != nil {
// 		return false, err
// 	}

// 	hashMap["CacheTTL"] = config.Redis.CacheDBDataTTL.String()
// 	ret := r.pool.EvalSha(ctx, sha, []string{key}, hashMap)
// 	if result, err := ret.Result(); err != nil {
// 		fmt.Println(result)
// 		log.Fatalf("Execute Redis fail: %v", err.Error())
// 	} else {
// 		fmt.Println("")
// 		fmt.Printf("userid: %s, result: %s", key, result)
// 	}

// 	return true, nil
// }

// Close redis close
func (r *RedisClient) Close() error {
	return r.pool.Close()
}
