package modules

import "time"

// Lock redis
type Lock struct {
	Key           string
	RequestID     string
	TTL           time.Duration
	Timeout       time.Duration
	RetryInterval time.Duration
}

// RedisData write data
type RedisData struct {
	UserKey      string
	OrderKey     string
	HashMap      map[string]interface{}
	Amount       int
	RequestID    string
	RequestIDTTL time.Duration
	WallteOpKey  string
	OpAmtBefor   int
	OpAmtAfter   int
	OpTimeSec    float64
	OrderID      string
	// PostData     interface{}
	PostData *PostDatav2
}
