package leakybucket

import (
	"net/http"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
)

type Bucket struct {
	Account   string
	Rate, Cap int
}

type RateLimitHandler struct {
	limiter     *redis_rate.Limiter
	userbackets map[string]*Bucket
	mux         sync.RWMutex

	next http.Handler
}

func (h *RateLimitHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var account, isok = req.Context().Value("account").(string)
	if !isok {
		// todo: response error?
	}
	var bucket *Bucket
	if bucket, isok = h.userbackets[account]; !isok {
		// todo: response error?
	}

	allow, err := h.limiter.Allow(context.TODO(),
		account, redis_rate.Limit{
			Rate: bucket.Rate, Burst: bucket.Cap, Period: time.Second,
		})

	if err != nil {
		// todo: handle this redis error

	} else if allow.Allowed < 1 {
		res.WriteHeader(http.StatusForbidden)
		_, _ = res.Write([]byte(fmt.Sprintf("account:%s, url:%s, rate limit was triggered",
			account, req.URL.String())))
		return
	}

	h.next.ServeHTTP(res, req)
}

func (h *RateLimitHandler) getBucket(user string) *Bucket {
	h.mux.RLock()
	defer h.mux.RUnlock()
	b, isok := h.userbackets[user]

	if !isok {
		return nil
	}

	return &(*b)
}

func (h *RateLimitHandler) upsertBucket(bucket *Bucket) {
	h.mux.Lock()
	defer h.mux.Unlock()
	b, isok := h.userbackets[bucket.Account]
	if !isok {
		b = &Bucket{Account: bucket.Account}
		h.userbackets[bucket.Account] = b
	}
	b.Cap, b.Rate = bucket.Cap, bucket.Rate
}

var _ = (http.Handler)((*RateLimitHandler)(nil))

type FnListAccountBuckets func() ([]*Bucket, error)

func NewRateLimitHandler(redisEndPoint string, next http.Handler, ) (*RateLimitHandler, error) {
	if next == nil {
		return nil, fmt.Errorf("listBuckets and next.ServerHTTP is required")
	}

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisEndPoint,
	})
	_ = rdb.FlushDB(ctx).Err()

	h := &RateLimitHandler{limiter: redis_rate.NewLimiter(rdb),
		userbackets: make(map[string]*Bucket), next: next}

	return h, nil
}