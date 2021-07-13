package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

func (h *RateLimiter) JSONRPCProxyInject(in interface{}, out interface{}) {
	vin := reflect.ValueOf(in)
	rout := reflect.ValueOf(out).Elem()

	for i := 0; i < vin.NumField(); i++ {
		method := vin.Type().Field(i).Name

		if vin.Field(i).Type().Kind() == reflect.Struct {
			h.JSONRPCProxyInject(vin.Field(i).Interface(), out)
			continue
		}

		field, exists := rout.Type().FieldByName(method)

		if !exists || field.Type.Kind() != reflect.Func {
			continue
		}

		fn := vin.FieldByName(method)
		if fn.IsNil() || fn.Kind() != reflect.Func {
			continue
		}

		rout.FieldByName(method).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
			ctx := args[0].Interface().(context.Context)

			host, _ := h.HostFromCtx(ctx)
			user, isok := h.AccFromCtx(ctx)

			if !isok { // todo: response error?
				h.Warnf("rate-limit, get user(host=%s, method=%s) failed: can't find an 'account' key\n",
					host, method)
				fn.Call(args)
				return
			}

			var limit, err = h.getUserLimit(user)
			if err != nil {
				// todo: response error?
				h.Warnf("rate-limit, get user(user=%s, host=%s, method=%s)limit failed: %s\n", user, host, method, err.Error())
				fn.Call(args)
				return
			}

			if limit.Cap == 0 {
				h.Infof("rate-limit, user=%s, host=%s, method=%s, have no request rate limit", limit.Account, host, method)
			} else {
				used, resetDur, allow := h.limiter.Allow(user, limit.Cap, limit.Duration)
				if allow {
					h.Infof("rate-limit, user=%s, host=%s, method=%s, cap=%d, used=%d,reset in %.2f(m)",
						user, host, method, limit.Cap, used, resetDur.Minutes())
				} else {
					if used == 0 {
						h.Warnf("rate-limit,user=%s, host=%s, method=%s,please check if redis-service is on,request-limit:cap=%d, used=%d, but returned allow is 'false'",
							user, host, method, limit.Cap, used)
					} else {
						message := fmt.Sprintf("rate-limit,user:%s, host:%s, method:%s is limited, cap=%d, used=%d,will reset in %.2f(m)",
							user, host, method, limit.Cap, used, resetDur.Minutes())
						h.Warn(message)
						err = errors.New(message)
						goto ABORT
					}
				}

			}
			return fn.Call(args)

		ABORT:
			rerr := reflect.ValueOf(&err).Elem()
			if fn.Type().NumOut() == 2 {
				return []reflect.Value{
					reflect.Zero(fn.Type().Out(0)),
					rerr,
				}
			}
			return []reflect.Value{rerr}
		}))
	}
}
