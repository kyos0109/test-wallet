package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// _ "net/http/pprof"

	"github.com/gin-gonic/gin"
)

const (
	redisDelimiter         = ":"
	redisUserPerfix        = "Users"
	redisOrderPerfix       = "Orders"
	redisLockOpsPerfix     = "LockOps"
	rediswallteOpsPerfix   = "walletOps"
	redisHashWalletKey     = "wallet"
	redisHashlastChangeKey = "lastChange"
	redisHashlastGameKey   = "lastGameID"
	residLockTTL           = 3600
	redisLockTimeout       = 30
	redisLockRetryInterval = 1

	postDeductCmd = "deduct"
	postSaveCmd   = "save"
)

var ctx = context.Background()

// PostData from request json
type PostData struct {
	Agent     string `json:"agent" validate:"required" binding:"required"`
	User      string `json:"user" validate:"required" binding:"required"`
	RequestID string `json:"requestid" validate:"required" binding:"required"`
	Amount    int    `json:"amount" validate:"required" binding:"required"`
	Action    string `json:"action" validate:"required" binding:"required"`
	Token     string `json:"token" validate:"required" binding:"required"`
}

// PostDeduct ...
type PostDeduct struct {
	PostData
	GameID int `json:"gameid" binding:"required"`
}

// PostSave ...
type PostSave struct {
	PostData
}

// RedisData write data
type RedisData struct {
	userKey      string
	orderKey     string
	hashMap      map[string]interface{}
	amount       int
	requestID    string
	requestIDTTL time.Duration
	wallteOpKey  string
	OpAmtBefor   int
	OpAmtAfter   int
	OpTimeSec    float64
	OrderID      string
	PostData     interface{}
}

func main() {

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	r := GetRedisClientInstance()
	defer r.Close()

	v1 := router.Group("/v1")
	{
		v1.POST("/api", chinSelectCaseFunc)
	}

	v2 := router.Group("/v2")
	{
		v2.POST("/api", chinSelectFunc)
	}

	v3 := router.Group("/v3")
	{
		v3.POST("/api/deduct", DeductWalletController)
		v3.POST("/api/save", SaveWalletController)
	}

	ws := router.Group("/ws")
	{
		ws.GET("", WsWallte)
	}

	tools := router.Group("/tools")
	{
		tools.POST("/fakedata", CreateRedisData)
	}

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// go func() {
	// 	http.ListenAndServe("0.0.0.0:6060", nil)
	// }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown: ", err)
	}

	log.Println("Server exiting")
}
