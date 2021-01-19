package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/gin-gonic/gin"
	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/database"
	"github.com/kyos0109/test-wallet/ginprometheus"
	"github.com/kyos0109/test-wallet/kredis"
	"github.com/kyos0109/test-wallet/wallet"
	"github.com/kyos0109/test-wallet/worker"
)

var (
	ctx                       context.Context
	cancel                    context.CancelFunc
	help, debug               bool
	configPath                string
	redisCtx, dbCtx           context.Context
	httpPort                  int
	readTimeout, writeTimeout time.Duration
)

func init() {
	setupFlag()
	printServiceConfigSet()

	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
}

func main() {
	pctx := context.Background()
	initWokerAndConfig(&pctx)

	router := gin.New()
	gp := ginprometheus.New(router, func(gp *ginprometheus.GinPrometheus) {
		gp.RedisCtx = redisCtx
	})

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(gp.Middleware())

	routerEntry(router)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(httpPort),
		Handler:      router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Println("Shutdown Server ...")
	ctx, cancel := context.WithTimeout(pctx, 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown: ", err)
	}

	log.Println("Server exiting")
}

func printServiceConfigSet() {
	log.Print("*********************************************")
	log.Printf("Service Config: %s", configPath)
	log.Printf("Redis Request ID TTL: %d sec", int64(config.Redis.RequestIDTTL/time.Second))
	log.Printf("Redis Cache DB Data TTL: %d sec", int64(config.Redis.CacheDBDataTTL/time.Second))
	log.Printf("Redis Orders Data TTL: %d sec", int64(config.Redis.OrdersDataTTL/time.Second))
	log.Printf("Redis Lock TTL: %d sec", int64(config.Redis.LockTTL/time.Second))
	log.Printf("Redis Lock Timeout: %d sec", int64(config.Redis.LockTimeout/time.Second))
	log.Printf("Redis Lock Retry Interval: %d sec", int64(config.Redis.LockRetryInterval/time.Second))
	log.Printf("Redis Connect Pool Size: %d ", config.Redis.ConnectPool)
	log.Printf("Redis Connect Retry: %d ", config.Redis.ConnectReTry)
	log.Printf("Database Max Connect Pool Size: %d", config.DB.SetMaxOpenConns)
	log.Printf("Database Max Connect Idle Size: %d", config.DB.SetMaxIdleConns)
	log.Printf("Database Connect Max Idle Time: %d sec", int64(config.DB.SetConnMaxIdleTime/time.Second))
	log.Print("*********************************************")
}

func setupFlag() {
	flag.BoolVar(&help, "h", false, "This help")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.IntVar(&config.Worker.DBSyncNums, "w", config.Worker.DBSyncNums, "Start DB Sync Worker Number")
	flag.IntVar(&config.DB.SetMaxOpenConns, "db-pool-conn", config.DB.SetMaxOpenConns, "Database Connect Pool Size")
	flag.IntVar(&config.DB.SetMaxIdleConns, "db-idle-conn", config.DB.SetMaxIdleConns, "Database Connect Idle Size")
	flag.IntVar(&config.Redis.ConnectPool, "redis-pool-conn", config.Redis.ConnectPool, "Redis Connect Pool Size")
	flag.IntVar(&config.Redis.ConnectReTry, "redis-retry-conn", config.Redis.ConnectReTry, "Redis Connect Retry")
	flag.StringVar(&configPath, "c", "config.yaml", "config path")
	flag.DurationVar(&config.DB.SetConnMaxIdleTime, "db-timeout", config.DB.SetConnMaxIdleTime, "Database Connect Max Idle Time")
	flag.DurationVar(&config.Redis.RequestIDTTL, "req-ttl", config.Redis.RequestIDTTL, "Request ID List TTL")
	flag.DurationVar(&config.Redis.CacheDBDataTTL, "cache-ttl", config.Redis.CacheDBDataTTL, "Redis Cache DB Data TTL")
	flag.DurationVar(&config.Redis.OrdersDataTTL, "order-ttl", config.Redis.OrdersDataTTL, "Redis Order Data TTL, Default UnLimit")
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}
}

func initWokerAndConfig(ctx *context.Context) {
	c, err := config.ReadConfig(configPath)
	if err != nil {
		log.Fatalln(err)
	}

	httpPort = c.HTTPConfig.Port
	readTimeout = c.HTTPConfig.ReadTimeout
	writeTimeout = c.HTTPConfig.WriteTimeout

	dbCtx = database.InitWithCtx(ctx, &c.DatabaseConfig)
	redisCtx = kredis.InitWithCtx(ctx, &c.RedisConfig)

	go wallet.ExpiryWorker(dbCtx)

	log.Print("Start Worker: ", config.Worker.DBSyncNums)
	for i := 0; i < config.Worker.DBSyncNums; i++ {
		go worker.SyncDBWorker(dbCtx, redisCtx)
	}
}
