package main

import (
	"errors"
	"net/http"
	"sync"
	"time"

	guuid "github.com/google/uuid"
)

// WalletEntry ...
func WalletEntry(rd *RedisData) (int, error) {
	var (
		l  Lock
		wg sync.WaitGroup
	)

	timer := time.Now()

	rd.hashMap = make(map[string]interface{})

	r := GetRedisClientInstance()

	rd.OrderID = guuid.New().String()
	rd.requestIDTTL = 60

	l.ttl = residLockTTL
	l.timeout = redisLockTimeout
	l.retryInterval = redisLockRetryInterval

	switch p := rd.PostData.(type) {
	case *PostSave:
		rd.requestID = p.RequestID
		rd.userKey = BuildRedisDataWithDelimiter(redisUserPerfix, &p.Agent, &p.User)
		rd.wallteOpKey = BuildRedisDataWithDelimiter(rediswallteOpsPerfix, &p.Agent, &p.User)

		ok, err := r.SetRequestIDLog(rd)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !ok {
			return http.StatusPreconditionFailed, errors.New("Request ID Repeat")
		}

		l.key = BuildRedisDataWithDelimiter(redisLockOpsPerfix, &p.Agent, &p.User)
		l.RequestID = p.RequestID

		err = r.TryLock(&l)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer r.UnLock(&l)

		accountBalance := r.UserWalletHGet(rd)
		if accountBalance < 0 {
			return http.StatusOK, errors.New("Not got Balance")
		}

		rd.OpAmtBefor = accountBalance

		rd.amount = accountBalance + (p.Amount)

		rd.hashMap[redisHashWalletKey] = rd.amount
		rd.hashMap[redisHashlastChangeKey] = time.Now()

	case *PostDeduct:
		rd.requestID = p.RequestID
		rd.userKey = BuildRedisDataWithDelimiter(redisUserPerfix, &p.Agent, &p.User)
		rd.wallteOpKey = BuildRedisDataWithDelimiter(rediswallteOpsPerfix, &p.Agent, &p.User)

		ok, err := r.SetRequestIDLog(rd)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !ok {
			return http.StatusPreconditionFailed, errors.New("Request ID Repeat")
		}

		l.key = BuildRedisDataWithDelimiter(redisLockOpsPerfix, &p.Agent, &p.User)
		l.RequestID = p.RequestID

		err = r.TryLock(&l)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer r.UnLock(&l)

		accountBalance := r.UserWalletHGet(rd)
		if accountBalance < 0 {
			return http.StatusOK, errors.New("Not got Balance")
		}

		if p.Amount > accountBalance {
			return http.StatusOK, errors.New("Not Enough Balance")
		}

		rd.OpAmtBefor = accountBalance

		rd.amount = accountBalance + (-p.Amount)

		rd.hashMap[redisHashWalletKey] = rd.amount
		rd.hashMap[redisHashlastChangeKey] = time.Now()
		rd.hashMap[redisHashlastGameKey] = p.GameID

	default:
		return http.StatusInternalServerError, errors.New("interface error")

	}

	err := r.UserWalletHMSet(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	rd.OpAmtAfter = r.UserWalletHGet(rd)
	rd.OpTimeSec = time.Now().Sub(timer).Seconds()

	wg.Add(1)
	go func() {
		defer wg.Done()
		LogBalance(rd)
	}()

	wg.Wait()
	return http.StatusOK, nil
}
