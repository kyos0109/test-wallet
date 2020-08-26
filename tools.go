package main

import (
	"bytes"
	"log"
	"reflect"
)

// BuildRedisDataWithDelimiter create with : string
func BuildRedisDataWithDelimiter(args ...interface{}) string {
	var buffer bytes.Buffer

	for _, v := range args {
		switch val := v.(type) {
		case string:
			buffer.WriteString(val)
			buffer.WriteString(redisDelimiter)
		case *string:
			buffer.WriteString(*val)
			buffer.WriteString(redisDelimiter)
		default:
			log.Fatalln("Not match condition process, type:", reflect.TypeOf(val))
		}
	}
	return buffer.String()[:len(buffer.String())-1]
}

// LogBalance to redis
func LogBalance(rd *RedisData) {
	r := GetRedisClientInstance()
	r.LPush(rd)

	rd.orderKey = BuildRedisDataWithDelimiter(redisOrderPerfix, &rd.OrderID)
	r.SetOrderlog(rd)
}
