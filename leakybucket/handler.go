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

type CachedBuckets struct {
	Bucket
	expire time.Time
}

func (cb *CachedBuckets) expired() bool {
	return time.Now().Before(cb.expire)
}

type IBucketsFinder interface {
	UserBucket(string) (*Bucket, error)
	ListUserBuckets() ([]*Bucket, error)
}

type IValueFromCtx interface {
	AccFromCtx(context.Context) (string, bool)
	HostFromCtx(context.Context) (string, bool)
}

type FnAccFromCtx func(context.Context) (string, bool)

type ILoger interface {
	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Debug(args ...interface{})
	Debugf(template string, args ...interface{})
}

type RateLimitHandler struct {
	ILoger
	IValueFromCtx
	limiter       *redis_rate.Limiter
	cachedBuckets map[string]*CachedBuckets
	mux           sync.RWMutex
	next          http.Handler
	finder        IBucketsFinder

	refreshTaskRunning bool
}

func (h *RateLimitHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if h.IValueFromCtx == nil { // todo: response error?
		h.next.ServeHTTP(res, req)
		return
	}
	reqCtx := req.Context()

	host, _ := h.HostFromCtx(reqCtx)
	user, isok := h.AccFromCtx(reqCtx)

	if !isok { // todo: response error?
		h.Warnf("rate-limit, get user(request from host:%s) failed: can't find an 'account' key\n", host)
		h.next.ServeHTTP(res, req)
		return
	}

	var bucket, err = h.getBucket(user)
	if err != nil {
		// todo: response error?
		h.Warnf("rate-limit, get user(%s, host:%s)buckets failed: %s\n", user, host, err.Error())
		h.next.ServeHTTP(res, req)
		return
	}

	if bucket.Rate == 0 || bucket.Cap == 0 {
		h.Infof("rate-limit, user:%s, have no request rate limit", bucket.Account)
	} else {
		var allow *redis_rate.Result

		if allow, err = h.limiter.Allow(context.TODO(), user,
			redis_rate.Limit{Rate: bucket.Rate, Burst: bucket.Cap, Period: time.Second}); err != nil {
			// todo: handle this redis error
			h.Warnf("rate-limit, user:%s, host:%s, error:%s\n", user, host, err.Error())
			h.next.ServeHTTP(res, req)
			return
		}

		h.Infof("rate-limit, user:%s, host:%s, allow:%d, limit-burst:%d, limit-rate:%d, remaining:%d",
			user, host, allow.Allowed, allow.Limit.Burst, allow.Limit.Rate, allow.Remaining)

		if allow.Allowed < 1 {
			message := fmt.Sprintf("account:%s,url:%s,request is limited,retry after:%.2f(seconds)\n",
				user, req.URL.String(),
				allow.RetryAfter.Seconds())
			res.WriteHeader(http.StatusForbidden)
			h.Warnf(message)
			_, _ = res.Write([]byte(message))
			return
		}
	}

	h.next.ServeHTTP(res, req)
}

func (h *RateLimitHandler) getBucket(user string) (*Bucket, error) {
	var cache *CachedBuckets
	var err error
	var isok bool
	var bucket *Bucket

	h.mux.RLock()
	cache, isok = h.cachedBuckets[user]
	h.mux.RUnlock()

	if isok && !cache.expired() {
		return &(*(&cache.Bucket)), nil
	}

	// todo: may updated in multi-goroutine at the same time,
	//  but it's acceptable, have better approach?
	if bucket, err = h.finder.UserBucket(user); err != nil {
		return nil, err
	}

	h.mux.Lock()
	if isok {
		cache.Bucket = *bucket
	} else {
		cache = &CachedBuckets{Bucket: *bucket}
		h.cachedBuckets[cache.Account] = cache
	}

	cache.expire = time.Now().Add(time.Minute)
	h.mux.Unlock()

	return bucket, nil
}

// deprecated..getBucket task a better way to updating user buckets.
func (h *RateLimitHandler) StartRefreshBuckets() (closer func(), alreadyRunning bool) {
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.refreshTaskRunning {
		alreadyRunning = true
		return
	}
	h.refreshTaskRunning = true
	timer := time.NewTimer(time.Minute)
	ch := make(chan interface{}, 1)
	go func() {
		for {
			select {
			case <-timer.C:
				refreshTime := time.Now().Format("YY:HH:MM-mm:hh:ss")
				if err := h.refreshBuckets(); err != nil {
					h.Errorf("refresh user buckets(at:%s) failed:%s\n", refreshTime, err.Error())
					break
				}
				h.Infof("refresh user buckets at:%s, success!", refreshTime)

			case <-ch:
				h.refreshTaskRunning = false
				return
			}
		}
	}()
	return func() { close(ch) }, false
}

func (h *RateLimitHandler) refreshBuckets() error {
	buckets, err := h.finder.ListUserBuckets()
	if err != nil {
		return err
	}
	userBuckets := make(map[string]*CachedBuckets, len(buckets))
	for _, b := range buckets {
		userBuckets[b.Account] = &CachedBuckets{
			Bucket: *b,
			expire: time.Now().Add(time.Minute),
		}
	}

	h.mux.Lock()
	h.cachedBuckets = userBuckets
	h.mux.Unlock()

	return nil
}

func (authMux *RateLimitHandler) Warnf(template string, args ...interface{}) {
	if authMux.ILoger == nil {
		fmt.Printf("auth-middware warning:%s", fmt.Sprintf(template, args...))
		return
	}
	authMux.ILoger.Warnf(template, args...)
}

func (authMux *RateLimitHandler) Infof(template string, args ...interface{}) {
	if authMux.ILoger == nil {
		fmt.Printf("auth-midware info:%s", fmt.Sprintf(template, args...))
		return
	}
	authMux.ILoger.Infof(template, args...)
}

func (authMux *RateLimitHandler) Errorf(template string, args ...interface{}) {
	if authMux.ILoger == nil {
		fmt.Printf("auth-midware error:%s", fmt.Sprintf(template, args...))
		return
	}
	authMux.ILoger.Errorf(template, args...)
}

var _ = (http.Handler)((*RateLimitHandler)(nil))

func NewRateLimitHandler(redisEndPoint string, next http.Handler,
	valueFromCtx IValueFromCtx, finder IBucketsFinder, loger ILoger) (*RateLimitHandler, error) {
	if next == nil {
		return nil, fmt.Errorf("next.ServerHTTP is required")
	}

	if finder == nil || valueFromCtx == nil {
		return nil, fmt.Errorf("fnAccFromCtx and fnListBuckets is required")
	}

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisEndPoint,
	})

	if err := rdb.FlushDB(ctx).Err(); err != nil {
		return nil, err
	}
	h := &RateLimitHandler{
		ILoger:        loger,
		IValueFromCtx: valueFromCtx,
		finder:        finder,
		limiter:       redis_rate.NewLimiter(rdb),
		cachedBuckets: make(map[string]*Bucket), next: next}

	return h, nil
}
