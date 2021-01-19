package ginprometheus

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/kredis"
)

const (
	metricsPath     = "/metrics"
	faviconPath     = "/favicon.ico"
	fetchInterval   = time.Second * 10
	fetchTimeout    = time.Second * 15
	fetchRetryCount = 3
)

var (
	httpHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "http_server",
		Subsystem:   "",
		Name:        "requests_seconds",
		Help:        "Histogram of response latency (seconds) of http handlers.",
		ConstLabels: nil,
		Buckets:     nil,
	}, []string{"method", "code", "uri"})

	redisHistogram = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "redis",
		Subsystem:   "",
		Name:        "queue",
		Help:        "Queue System Status.",
		ConstLabels: nil,
	}, []string{"RedisKey", "FetchIntervalSec"})

	redisJobsCount          int64 = -1
	redisProcessingCount    int64 = -1
	redisFailedRequestCount int64 = -1
)

func init() {
	prometheus.MustRegister(httpHistogram, redisHistogram)
}

type handlerPath struct {
	sync.Map
}

func (hp *handlerPath) get(handler string) string {
	v, ok := hp.Load(handler)
	if !ok {
		return ""
	}
	return v.(string)
}

func (hp *handlerPath) set(ri gin.RouteInfo) {
	hp.Store(ri.Handler, ri.Path)
}

// GinPrometheus ...
type GinPrometheus struct {
	engine   *gin.Engine
	ignored  map[string]bool
	pathMap  *handlerPath
	updated  bool
	RedisCtx context.Context
}

// Option ...
type Option func(*GinPrometheus)

// Ignore ...
func Ignore(path ...string) Option {
	return func(gp *GinPrometheus) {
		for _, p := range path {
			gp.ignored[p] = true
		}
	}
}

// New new gin prometheus
func New(e *gin.Engine, options ...Option) *GinPrometheus {
	if e == nil {
		return nil
	}

	gp := &GinPrometheus{
		engine: e,
		ignored: map[string]bool{
			metricsPath: true,
			faviconPath: true,
		},
		pathMap: &handlerPath{},
	}

	for _, o := range options {
		o(gp)
	}

	go gp.fetchQueueStatusCount()

	return gp
}

func (gp *GinPrometheus) updatePath() {
	gp.updated = true
	for _, ri := range gp.engine.Routes() {
		gp.pathMap.set(ri)
	}
}

// Middleware set gin middleware
func (gp *GinPrometheus) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !gp.updated {
			gp.updatePath()
		}

		if gp.ignored[c.Request.URL.String()] {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		httpHistogram.WithLabelValues(
			c.Request.Method,
			strconv.Itoa(c.Writer.Status()),
			gp.pathMap.get(c.HandlerName()),
		).Observe(time.Since(start).Seconds())

		strFetchInterval := fetchInterval.String()
		redisHistogram.WithLabelValues(config.RedisJobsQueue, strFetchInterval).Set(float64(redisJobsCount))
		redisHistogram.WithLabelValues(config.RedisJobProcessing, strFetchInterval).Set(float64(redisProcessingCount))
		redisHistogram.WithLabelValues(config.RedisFailedRequestKey, strFetchInterval).Set(float64(redisFailedRequestCount))
	}
}

func (gp *GinPrometheus) fetchQueueStatusCount() {
	ticker := time.NewTicker(fetchInterval)
	count := 0

	for {
		select {
		case <-ticker.C:
			redisJobsCount = gp.redisLLen(config.RedisJobsQueue)
			redisProcessingCount = gp.redisLLen(config.RedisJobProcessing)
			redisFailedRequestCount = gp.redisLLen(config.RedisFailedRequestKey)
			break
		case <-time.After(fetchTimeout):
			count++
			log.Println("fetch redis metirce timeout")
			if count > fetchRetryCount {
				log.Printf("fetch redis metirce timeout total count: %d", count)
				log.Print("stop metrics worker")
				ticker.Stop()
				return
			}
			break
		}
	}
}

func (gp *GinPrometheus) redisLLen(key string) int64 {
	redis := gp.RedisCtx.Value(kredis.RedisClient{}).(*kredis.RedisClient)

	s, err := redis.LLen(key)
	if err == kredis.Nil {
		return -1
	}
	if err != nil {
		log.Println(err)
	}

	return s
}
