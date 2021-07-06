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
	next        http.Handler

	fnAccFromCtx func(ctx context.Context) (string, bool)
}

func (h *RateLimitHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if h.fnAccFromCtx == nil { // todo: response error?
		h.next.ServeHTTP(res, req)
		return
	}
	account, isok := h.fnAccFromCtx(req.Context())
	if !isok { // todo: response error?
		h.next.ServeHTTP(res, req)
		return
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
		fmt.Printf("rate limit allow error:%s\n", err.Error())
	} else {
		fmt.Printf("rate-limit: allow:%d, limit:%d, remaining:%d\n", allow.Allowed, allow.Limit, allow.Remaining)

		if allow.Allowed < 1 {
			res.WriteHeader(http.StatusForbidden)
			_, _ = res.Write([]byte(fmt.Sprintf("account:%s, url:%s, rate limit was triggered",
				account, req.URL.String())))
			return
		}
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

func NewRateLimitHandler(redisEndPoint string, next http.Handler, getAccount func(ctx context.Context) string) (*RateLimitHandler, error) {
	if next == nil {
		return nil, fmt.Errorf("listBuckets and next.ServerHTTP is required")
	}

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisEndPoint,
	})
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		return nil, err
	}
	h := &RateLimitHandler{limiter: redis_rate.NewLimiter(rdb),
		fnAccFromCtx: getAccount,
		userbackets:  make(map[string]*Bucket), next: next}

	return h, nil
}
