package worker

import (
	"context"
	"log"
	"time"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/database"
	"github.com/kyos0109/test-wallet/kredis"
	"github.com/kyos0109/test-wallet/modules"
)

// SyncDBWorker ...
func SyncDBWorker(dbCtx, redisCtx context.Context) {
	db := dbCtx.Value(database.DBConn{}).(*database.DBConn)
	redis := redisCtx.Value(kredis.RedisClient{}).(*kredis.RedisClient)

	for {
		select {
		case <-dbCtx.Done():
		case <-redisCtx.Done():
			log.Print("Stop worker...")
			return
		default:
			rd, err := redis.BRPopLPushJobResStruct()
			if err == kredis.Nil {
				break
			}
			if err != nil {
				log.Fatalln(err)
			}

			tx := db.Conn().Begin()

			r := tx.Create(&rd.Order)
			if r.Error != nil {
				tx.Rollback()
				log.Println(r.Error)
			}

			w := modules.Wallet{}
			rr := tx.Find(&w, &modules.Wallet{ID: rd.Order.WalletID})
			if rr.Error != nil {
				tx.Rollback()
				log.Println(rr.Error)
			}

			tx.Model(&w).Updates(&modules.Wallet{Amount: rd.Order.AfterAmount, UpdateAt: rd.Order.UpdateAt})

			tx.Commit()

			err = redis.LRemProcessing(rd, -1)
			if err != nil {
				log.Fatalln("lrem: ", err.Error())
			}

			if ok := redis.Expire(rd.OrderKey, config.Redis.OrdersDataTTL); !ok {
				log.Fatalln("Set Expire Order: ", ok)
			}

			rd.HashMap = make(map[string]interface{})
			rd.HashMap[config.RedisHashlastSyncKey] = time.Now()
			err = redis.UserWalletHMSet(rd)
			if err != nil {
				log.Fatalln(err)
			}
		}
	}
}
