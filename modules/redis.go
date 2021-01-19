package modules

// Lock redis
type Lock struct {
	Key       string
	RequestID string
}

// RedisData write data
type RedisData struct {
	UserKey     string
	OrderKey    string
	HashMap     map[string]interface{}
	RequestID   string
	WallteOpKey string
	OpTimeSec   float64
	OrderID     string
	PostData    *PostDatav2
	Order       *Order
}
