package config

import "time"

const (
	RedisDelimiter              = ":"
	RedisOrderPrefix            = "Orders"
	RedisUserPrefix             = "Users"
	RedisLockOpsPrefix          = "LockOps"
	RedisWallteOpsPrefix        = "WalletOps"
	RedisAgentPrefix            = "Agent"
	RedisHashWalletKey          = "Wallet"
	RedisHashWalletIDKey        = "WalletID"
	RedisHashlastChangeKey      = "LastChange"
	RedisHashlastSyncKey        = "LastSync"
	RedisHashlastGameKey        = "LastGameID"
	RedisHashlastStatusKey      = "Status"
	RedisHashAgentIDKey         = "AgentID"
	RedisHashPublicKey          = "PublicKey"
	RedisHashPrivateKey         = "PrivateKey"
	RedisHashSingleWalletURLKey = "SingleWalletURL"
	RedisHashAgentStatusKey     = "Status"
	RedisJobsQueue              = "Jobs"
	RedisJobProcessing          = "Processing"
	RedisReadSync               = "readySync"
	RedisFailedRequestKey       = "FailedRequest"
)

type redis struct {
	CacheDBDataTTL    time.Duration
	RequestIDTTL      time.Duration
	OrdersDataTTL     time.Duration
	LockTTL           time.Duration
	LockTimeout       time.Duration
	LockRetryInterval time.Duration
	ConnectPool       int
	ConnectReTry      int
}

var (
	Redis = &redis{
		CacheDBDataTTL:    1 * time.Hour,
		RequestIDTTL:      60 * time.Second,
		OrdersDataTTL:     15 * time.Minute,
		LockTTL:           1 * time.Hour,
		LockTimeout:       15 * time.Second,
		LockRetryInterval: 1 * time.Second,
		ConnectPool:       200,
		ConnectReTry:      3,
	}
)
