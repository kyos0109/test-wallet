package wallet

import (
	"bytes"
	"log"
	"reflect"
	"sync"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/kredis"
	"github.com/kyos0109/test-wallet/modules"
)

// BuildRedisDataWithDelimiter create with : string
func BuildRedisDataWithDelimiter(args ...interface{}) string {
	var buffer bytes.Buffer

	for _, v := range args {
		switch val := v.(type) {
		case string:
			buffer.WriteString(val)
			buffer.WriteString(config.RedisDelimiter)
		case *string:
			buffer.WriteString(*val)
			buffer.WriteString(config.RedisDelimiter)
		default:
			log.Fatalln("Not match condition process, type:", reflect.TypeOf(val))
		}
	}
	return buffer.String()[:len(buffer.String())-1]
}

// LogBalance to redis
func LogBalance(rd *modules.RedisData) {
	r := kredis.GetRedisClientInstance()
	rd.OrderKey = BuildRedisDataWithDelimiter(config.RedisOrderPrefix, &rd.OrderID)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r.LPush(rd)
	}()
	go func() {
		defer wg.Done()
		r.SetOrderlog(rd)
	}()
	wg.Wait()
}
