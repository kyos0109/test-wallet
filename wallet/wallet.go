package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	guuid "github.com/google/uuid"

	kredis "github.com/kyos0109/test-wallet/redis"

	"github.com/kyos0109/test-wallet/database"

	"github.com/kyos0109/test-wallet/modules"
)

type wallet struct {
	db   *database.DBConn
	data *modules.Wallet
	post *modules.PostDatav2
}

// Entry ...
func Entry(rd *modules.RedisData) (int, error) {
	var l modules.Lock

	timer := time.Now()

	rd.HashMap = make(map[string]interface{})

	r := kredis.GetRedisClientInstance()

	rd.OrderID = guuid.New().String()
	rd.RequestIDTTL = 60

	l.TTL = kredis.RedisLockTTL
	l.Timeout = kredis.RedisLockTimeout
	l.RetryInterval = kredis.RedisLockRetryInterval

	p := rd.PostData
	rd.RequestID = p.RequestID
	rd.UserKey = BuildRedisDataWithDelimiter(kredis.RedisUserPerfix, &p.Agent, &p.User)
	rd.WallteOpKey = BuildRedisDataWithDelimiter(kredis.RediswallteOpsPerfix, &p.Agent, &p.User)

	ok, err := r.SetRequestIDLog(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !ok {
		return http.StatusPreconditionFailed, errors.New("Request ID Repeat")
	}

	l.Key = BuildRedisDataWithDelimiter(kredis.RedisLockOpsPerfix, &p.Agent, &p.User)
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

	switch d := rd.PostData.Detail.(type) {
	case *modules.PostStorev2:
		rd.Amount = accountBalance + (p.Amount)
	case *modules.PostDeductv2:
		if p.Amount > accountBalance {
			return http.StatusOK, errors.New("Not Enough Balance")
		}

		rd.Amount = accountBalance + (-p.Amount)
		rd.HashMap[kredis.RedisHashlastGameKey] = d.GameID
	default:
		return http.StatusInternalServerError, errors.New("interface error")
	}

	rd.HashMap[kredis.RedisHashWalletKey] = rd.Amount
	rd.HashMap[kredis.RedisHashlastChangeKey] = time.Now()

	err = r.UserWalletHMSet(rd)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	rd.OpAmtAfter = r.UserWalletHGet(rd)
	rd.OpTimeSec = time.Now().Sub(timer).Seconds()

	LogBalance(rd)

	return http.StatusOK, nil
}

// EntryDB ...
func EntryDB(postData *modules.PostDatav2) (int, error) {
	w := &wallet{}
	w.db = database.GetDBInstance()
	w.post = postData

	if err := w.checkRequestID(); err != nil {
		return http.StatusBadRequest, errors.New(err.Error())
	}

	uid, _ := strconv.Atoi(postData.User)
	aid, _ := strconv.Atoi(postData.Agent)
	wallet := &modules.Wallet{}

	tx := w.db.Conn().Begin()

	r := w.db.Conn().
		Table("users").Select("wallets.*").
		Joins("left join wallets on wallets.user_id = users.id").
		Find(&wallet, modules.User{ID: uid, AgentID: aid})

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
	db := database.GetDBInstance()

	log.Print("start expiry worker...")
	for {
		select {
		case <-ctx.Done():
			log.Print("stop worker...")
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
