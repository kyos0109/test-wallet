package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	guuid "github.com/google/uuid"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/database"
	"github.com/kyos0109/test-wallet/kredis"
	"github.com/kyos0109/test-wallet/modules"
)

type wallet struct {
	db    *database.DBConn
	redis *kredis.RedisClient
	data  *modules.Wallet
	post  *modules.PostDatav2
}

// Entry ...
func Entry(rd *modules.RedisData) (int, error) {
	timer := time.Now()

	rd.OrderID = guuid.New().String()

	p := rd.PostData
	rd.RequestID = p.RequestID
	rd.UserKey = BuildRedisDataWithDelimiter(config.RedisUserPrefix, &p.Agent, &p.User)
	rd.WallteOpKey = BuildRedisDataWithDelimiter(config.RedisWallteOpsPrefix, &p.Agent, &p.User)

	w := &wallet{
		redis: kredis.GetRedisClientInstance(),
		db:    database.GetDBInstance(),
	}
	ok, err := w.redis.SetRequestIDLog(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !ok {
		return http.StatusPreconditionFailed, errors.New("Request ID Repeat")
	}

	uid, _ := strconv.Atoi(p.User)
	rd.Order = &modules.Order{
		ID:        guuid.MustParse(rd.OrderID),
		UserID:    uid,
		RequestID: p.RequestID,
		Status:    modules.OrderCreate,
		CreateAt:  time.Now(),
	}

	l := &modules.Lock{
		Key:       BuildRedisDataWithDelimiter(config.RedisLockOpsPrefix, &p.Agent, &p.User),
		RequestID: p.RequestID,
	}

	err = w.redis.TryLock(l)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer w.redis.UnLock(l)

LOOP:
	accountBalance := w.redis.UserWalletHGet(rd)
	if accountBalance < 0 {
		if err = w.onceSyncDBWalletToRedis(rd); err != nil {
			return http.StatusOK, errors.New("Not got Balance")
		}

		goto LOOP
	}
	if !w.redis.UserStatusHGet(rd) {
		return http.StatusOK, errors.New("User Status IS Disable")
	}

	walletID := w.redis.UserWalletIDHGet(rd)
	if walletID < 0 {
		return http.StatusOK, errors.New("Not got wallet id")
	}

	rd.Order.WalletID = walletID
	rd.Order.BeforeAmount = float64(accountBalance)
	rd.HashMap = make(map[string]interface{})

	switch d := rd.PostData.Detail.(type) {
	case *modules.PostStorev2:
		rd.Order.AfterAmount = float64(accountBalance + (p.Amount))

		rd.Order.OpType = modules.WalletStore
	case *modules.PostDeductv2:
		if p.Amount > accountBalance {
			return http.StatusOK, errors.New("Not Enough Balance")
		}

		rd.Order.AfterAmount = float64(accountBalance + (-p.Amount))
		rd.Order.OpType = modules.WalletDeduct
		rd.Order.GameID = d.GameID
		rd.HashMap[config.RedisHashlastGameKey] = d.GameID
	default:
		return http.StatusInternalServerError, errors.New("interface error")
	}

	rd.HashMap[config.RedisHashWalletKey] = rd.Order.AfterAmount
	rd.HashMap[config.RedisHashlastChangeKey] = time.Now()

	rd.Order.Status = modules.OrderOk
	rd.Order.UpdateAt = time.Now()

	err = w.redis.UserWalletHMSet(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	rd.OpTimeSec = time.Now().Sub(timer).Seconds()

	LogBalance(rd)

	if err = w.redis.LPushJob(rd); err != nil {
		log.Println(rd)
		log.Println(err)
	}

	return http.StatusOK, nil
}

// EntryDB ...
func EntryDB(postData *modules.PostDatav2) (int, error) {
	w := &wallet{
		db: database.GetDBInstance(),
	}
	w.post = postData

	if err := w.checkRequestID(); err != nil {
		return http.StatusBadRequest, errors.New(err.Error())
	}

	wallet := &modules.Wallet{}

	uid, _ := strconv.Atoi(w.post.User)

	tx := w.db.Conn().Begin()

	r := w.db.Conn().
		Table("users").Select("wallets.*").
		Joins("left join wallets on wallets.user_id = users.id").
		Find(&wallet, modules.User{ID: uid, AgentID: postData.Agent})

	if r.Error != nil {
		tx.Rollback()
		return http.StatusInternalServerError, errors.New("get user data error")
	}

	if r.RowsAffected <= 0 {
		tx.Rollback()
		return http.StatusOK, errors.New("user not found")
	}

	w.data = wallet
	oid, err := w.createOrder(modules.OrderCreate, postData)
	if err != nil {
		return http.StatusInternalServerError, errors.New("create order error")
	}

	order := &modules.Order{}
	o := w.db.Conn().Find(&order, &modules.Order{ID: oid})
	if o.Error != nil {
		return http.StatusOK, errors.New("get order error")
	}
	if o.RowsAffected <= 0 {
		return http.StatusOK, errors.New("not found order")
	}

	switch d := postData.Detail.(type) {
	case *modules.PostDeductv2:

		if float64(postData.Amount) > wallet.Amount {
			tx.Rollback()
			return http.StatusOK, errors.New("user coco not enough")
		}

		wallet.Amount = wallet.Amount - float64(postData.Amount)
		wallet.UpdateAt = time.Now()

		order.AfterAmount = wallet.Amount
		order.Status = modules.OrderOk
		order.OpType = modules.WalletDeduct
		order.GameID = d.GameID
		order.UpdateAt = time.Now()

		if tx.Save(&wallet).Error != nil {
			tx.Rollback()
			return http.StatusOK, errors.New("update user balance error")
		}
		if tx.Save(&order).Error != nil {
			tx.Rollback()
			return http.StatusOK, errors.New("update user order error")
		}

	case *modules.PostStorev2:
		wallet.Amount = wallet.Amount + float64(postData.Amount)
		wallet.UpdateAt = time.Now()

		order.AfterAmount = wallet.Amount
		order.Status = modules.OrderOk
		order.OpType = modules.WalletStore
		order.UpdateAt = time.Now()

		if tx.Save(&wallet).Error != nil {
			tx.Rollback()
			return http.StatusOK, errors.New("update user balance error")
		}
		if tx.Save(&order).Error != nil {
			tx.Rollback()
			return http.StatusOK, errors.New("update user order error")
		}

	default:
		tx.Rollback()
		return http.StatusInternalServerError, errors.New("interface error")

	}

	tx.Commit()
	return http.StatusOK, nil
}

func (w *wallet) createOrder(oType modules.OrderStatus, p *modules.PostDatav2) (guuid.UUID, error) {
	o := &modules.Order{
		UserID:       w.data.UserID,
		WalletID:     w.data.ID,
		RequestID:    p.RequestID,
		OpType:       modules.WalletNone,
		BeforeAmount: w.data.Amount,
		Status:       oType,
		CreateAt:     time.Now(),
		UpdateAt:     time.Now(),
	}

	r := w.db.Conn().Create(&o)
	if r.Error != nil {
		return guuid.Nil, r.Error
	}

	return o.ID, nil
}

func (w *wallet) checkRequestID() error {
	a := &modules.APIRequestIDs{}
	u := guuid.MustParse(w.post.RequestID)
	p := w.post.ClientIP

	r := w.db.Conn().Find(&a, &modules.APIRequestIDs{ID: u, IP: p})
	if r.Error != nil {
		return r.Error
	}

	if r.RowsAffected == 0 {
		req := &modules.APIRequestIDs{
			ID:       u,
			IP:       p,
			CreateAt: time.Now(),
		}
		rr := w.db.Conn().Create(&req)
		if rr.Error != nil {
			return r.Error
		}
	} else {
		return errors.New("Request ID Repeat: " + a.ID.String() + " from: " + a.IP)
	}

	return nil
}

// ExpiryWorker ...
func ExpiryWorker(ctx context.Context) {
	db := ctx.Value(database.DBConn{}).(*database.DBConn)

	log.Print("start expiry worker...")
	for {
		select {
		case <-ctx.Done():
			log.Print("Stop worker...")
			return
		default:
			r := db.Conn().Where("create_at < ?", time.Now()).Delete(&modules.APIRequestIDs{})
			if r.Error != nil {
				fmt.Println(r.Error)
			}
			time.Sleep(time.Second * 60)
		}
	}
}

func (w *wallet) onceSyncDBWalletToRedis(rd *modules.RedisData) error {
	wallet := &modules.Wallet{}
	agent := &modules.Agent{}

	uid, _ := strconv.Atoi(rd.PostData.User)
	tx := w.db.Conn().Begin()

	a := tx.Select("id").Find(&agent, modules.Agent{ID: rd.PostData.Agent, Status: true})
	if a.RowsAffected <= 0 {
		return errors.New("agent not found or disable")
	}

	r := tx.Table("users").Select("wallets.*").
		Joins("INNER JOIN wallets ON wallets.user_id = users.id").
		Find(&wallet, modules.User{ID: uid, AgentID: rd.PostData.Agent, Status: true})

	if r.Error != nil {
		tx.Rollback()
		return errors.New("get user data error")
	}

	if r.RowsAffected <= 0 {
		tx.Rollback()
		return errors.New("user not found")
	}

	tx.Commit()

	rd.HashMap = make(map[string]interface{})
	rd.HashMap[config.RedisHashWalletKey] = wallet.Amount
	rd.HashMap[config.RedisHashWalletIDKey] = wallet.ID
	rd.HashMap[config.RedisHashlastSyncKey] = time.Now()
	rd.HashMap[config.RedisHashlastStatusKey] = true

	if err := w.redis.UserWalletHMSet(rd); err != nil {
		return err
	}

	if ok := w.redis.Expire(rd.UserKey, config.Redis.CacheDBDataTTL); !ok {
		return errors.New("Set Expire Cache User Data Error")
	}
	// _, err := w.redis.SyncDBCache(rd.UserKey, rd.HashMap)
	// if err != nil {
	// 	return err
	// }

	return nil
}

// EntrySingle ...
func EntrySingle(rd *modules.RedisData) (int, error) {
	timer := time.Now()

	rd.OrderID = guuid.New().String()

	p := rd.PostData
	rd.RequestID = p.RequestID
	rd.UserKey = BuildRedisDataWithDelimiter(config.RedisUserPrefix, &p.Agent, &p.User)
	rd.WallteOpKey = BuildRedisDataWithDelimiter(config.RedisWallteOpsPrefix, &p.Agent, &p.User)

	w := &wallet{
		redis: kredis.GetRedisClientInstance(),
		db:    database.GetDBInstance(),
	}

	ok, err := w.redis.SetRequestIDLog(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !ok {
		return http.StatusPreconditionFailed, errors.New("Request ID Repeat")
	}

	uid, _ := strconv.Atoi(p.User)
	rd.Order = &modules.Order{
		ID:        guuid.MustParse(rd.OrderID),
		UserID:    uid,
		RequestID: p.RequestID,
		Status:    modules.OrderCreate,
		CreateAt:  time.Now(),
	}

	l := &modules.Lock{
		Key:       BuildRedisDataWithDelimiter(config.RedisLockOpsPrefix, &p.Agent, &p.User),
		RequestID: p.RequestID,
	}

	err = w.redis.TryLock(l)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer w.redis.UnLock(l)

LOOP1:
	accountBalance := w.redis.UserWalletHGet(rd)
	if accountBalance < 0 {
		if err = w.onceSyncDBWalletToRedis(rd); err != nil {
			return http.StatusOK, err
		}

		goto LOOP1
	}
	if !w.redis.UserStatusHGet(rd) {
		return http.StatusOK, errors.New("User Status is Disable")
	}

	walletID := w.redis.UserWalletIDHGet(rd)
	if walletID < 0 {
		return http.StatusOK, errors.New("Not got wallet id")
	}

	rd.Order.WalletID = walletID
	rd.Order.BeforeAmount = float64(accountBalance)

LOOP2:
	url := w.redis.HGet(BuildRedisDataWithDelimiter(config.RedisAgentPrefix, p.Agent), config.RedisHashSingleWalletURLKey)
	if url == "" {
		if err := w.onceSyncAgentToRedis(rd.PostData); err != nil {
			return http.StatusOK, err
		}
		goto LOOP2
	}

	postData, err := json.Marshal(rd.PostData)
	if err != nil {
		return http.StatusBadRequest, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return http.StatusInternalServerError, err
	}

	client := &http.Client{}
	client.Timeout = config.Http.ClientTimeout

	resp, err := client.Do(req)
	if err != nil {
		return http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 199 || resp.StatusCode > 399 {
		return http.StatusGone, errors.New("Wallet URL Response Status Code: " + resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}

	log.Println(string(body))

	res := &modules.EchoResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}

	rd.HashMap = make(map[string]interface{})

	switch d := rd.PostData.Detail.(type) {
	case *modules.PostStorev2:
		rd.Order.AfterAmount = float64(res.Echo.Amount)
		rd.Order.OpType = modules.WalletStore
	case *modules.PostDeductv2:
		rd.Order.OpType = modules.WalletDeduct
		rd.Order.AfterAmount = float64(res.Echo.Amount)
		rd.Order.GameID = d.GameID
		rd.HashMap[config.RedisHashlastGameKey] = d.GameID
	default:
		return http.StatusInternalServerError, errors.New("interface error")
	}

	rd.HashMap[config.RedisHashWalletKey] = res.Echo.Amount
	rd.HashMap[config.RedisHashlastChangeKey] = time.Now()

	rd.Order.Status = modules.OrderOk
	rd.Order.UpdateAt = time.Now()

	err = w.redis.UserWalletHMSet(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	rd.OpTimeSec = time.Now().Sub(timer).Seconds()

	LogBalance(rd)

	if err = w.redis.LPushJob(rd); err != nil {
		log.Println(rd)
		log.Println(err)
	}

	return http.StatusOK, nil
}

func (w *wallet) onceSyncAgentToRedis(post *modules.PostDatav2) error {
	agent := &modules.Agent{}

	res := w.db.Conn().Find(&agent, modules.Agent{ID: post.Agent, Status: true})
	if res.RowsAffected <= 0 {
		return errors.New("Agent not found or disable")
	}

	if len(agent.SingleWalletUrl) <= 0 {
		return errors.New("Single Wallet Url Not Set")
	}

	agentHash := make(map[string]interface{})
	agentHash[config.RedisHashAgentIDKey] = agent.ID
	agentHash[config.RedisHashPublicKey] = agent.Key
	agentHash[config.RedisHashPrivateKey] = agent.PrivateKey
	agentHash[config.RedisHashSingleWalletURLKey] = agent.SingleWalletUrl
	agentHash[config.RedisHashAgentStatusKey] = agent.Status

	agentKey := BuildRedisDataWithDelimiter(config.RedisAgentPrefix, post.Agent)
	if err := w.redis.HMSet(agentKey, agentHash); err != nil {
		return errors.New("Agent Data Sync Error")
	}

	if ok := w.redis.Expire(agentKey, config.Redis.CacheDBDataTTL); !ok {
		return errors.New("Set Expire Cache Agent Data Error")
	}

	return nil
}
