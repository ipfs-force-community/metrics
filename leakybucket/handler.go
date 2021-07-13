package leakybucket

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/go-redis/redis_rate/v7"
)

type Bucket struct {
	Account   string
	Rate, Cap int
}

type Limit struct {
	Account  string
	Cap      int64
	Duration time.Duration
}

type ILimitFinder interface {
	GetUserLimit(string) (*Limit, error)
	ListUserLimits() ([]*Limit, error)
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
	limiter   *redis_rate.Limiter
	userLimit map[string]*Limit
	mux       sync.RWMutex
	next      http.Handler
	finder    ILimitFinder

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

	var limit, err = h.getUserLimit(user)
	if err != nil {
		// todo: response error?
		h.Warnf("rate-limit, get user(%s, host:%s)limit failed: %s\n", user, host, err.Error())
		h.next.ServeHTTP(res, req)
		return
	}

	if limit.Cap == 0 {
		h.Infof("rate-limit, user:%s, have no request rate limit", limit.Account)
	} else {
		used, resetDur, allow := h.limiter.Allow(user, limit.Cap, limit.Duration)
		if allow {
			h.Infof("rate-limit, user=%s, host=%s, cap=%d, used=%d, will reset in %.2f(m)",
				user, host, limit.Cap, used, resetDur.Minutes())
		} else {
			if used == 0 {
				h.Warnf("rate-limit,please check if redis-service is on,request-limit:cap=%d,used=%d, but returned allow is 'false'")
			} else {
				h.Warnf("rate-limit,user:%s, host:%s request is limited, cap=%d, used=%d,will reset in %.2f(m)",
					user, host, limit.Cap, used, resetDur.Minutes())
				if err = rpcError(res, user, host, limit.Cap, used, resetDur); err != nil {
					_, _ = res.Write([]byte(err.Error()))
				}
				res.WriteHeader(http.StatusForbidden)
				return
			}
		}

	}

	h.next.ServeHTTP(res, req)
}

func (h *RateLimitHandler) getUserLimit(user string) (*Limit, error) {
	// todo: use h.userLimit as cache, and refresh it periodically
	return h.finder.GetUserLimit(user)
}

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
	limits, err := h.finder.ListUserLimits()
	if err != nil {
		return err
	}
	userLimits := make(map[string]*Limit, len(limits))
	for _, b := range limits {
		userLimits[b.Account] = b
	}

	h.mux.Lock()
	h.userLimit = userLimits
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
	valueFromCtx IValueFromCtx, finder ILimitFinder, loger ILoger) (*RateLimitHandler, error) {
	if next == nil {
		return nil, fmt.Errorf("next.ServerHTTP is required")
	}

	if finder == nil || valueFromCtx == nil {
		return nil, fmt.Errorf("fnAccFromCtx and fnListBuckets is required")
	}

	h := &RateLimitHandler{
		ILoger:        loger,
		IValueFromCtx: valueFromCtx,
		finder:        finder,
		userLimit:     make(map[string]*Limit),
		next:          next,
		limiter: redis_rate.NewLimiter(
			redis.NewRing(&redis.RingOptions{
				Addrs: map[string]string{"server1": redisEndPoint},
			}),
		)}

	return h, nil
}
