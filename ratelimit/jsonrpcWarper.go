package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"go.opencensus.io/trace"
	"reflect"
)

func (h *RateLimiter) CallProxy(fname string, fn reflect.Value, args []reflect.Value) []reflect.Value {
	ctx := args[0].Interface().(context.Context)

	host, _ := h.HostFromCtx(ctx)
	user, isok := h.AccFromCtx(ctx)

	span := trace.FromContext(ctx)
	if span == nil {
		ctx, span = trace.StartSpan(ctx, "api.handle")
	}

	span.AddAttributes(trace.StringAttribute("account", user))

	if !isok { // todo: response error?
		h.Warnf("rate-limit, get user(host=%s, method=%s) failed: can't find an 'account' key\n",
			host, fname)
		return fn.Call(args)
	}

	var limit, err = h.getUserLimit(user, "", "")
	if err != nil {
		// todo: response error?
		h.Warnf("rate-limit, get user(user=%s, host=%s, method=%s)limit failed: %s\n", user, host, fname, err.Error())

		return fn.Call(args)
	}

	if limit.Cap == 0 {
		h.Debugf("rate-limit, user=%s, host=%s, method=%s, have no request rate limit", limit.Account, host, fname)
	} else {
		used, resetDur, allow := h.limiter.Allow(user, limit.Cap, limit.Duration)
		if allow {
			h.Debugf("rate-limit, user=%s, host=%s, method=%s, cap=%d, used=%d,reset in %.2f(m)",
				user, host, fname, limit.Cap, used, resetDur.Minutes())
		} else {
			if used == 0 {
				h.Warnf("rate-limit,user=%s, host=%s, method=%s,please check if redis-service is on,request-limit:cap=%d, used=%d, but returned allow is 'false'",
					user, host, fname, limit.Cap, used)
			} else {
				message := fmt.Sprintf("rate-limit,user:%s, host:%s, method:%s is limited, cap=%d, used=%d,will reset in %.2f(m)",
					user, host, fname, limit.Cap, used, resetDur.Minutes())
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
}

func (h *RateLimiter) WraperLimiter(in interface{}, out interface{}) {
	vin := reflect.ValueOf(in)
	rout := reflect.ValueOf(out).Elem()

	for i := 0; i < vin.NumField(); i++ {
		fieldName := vin.Type().Field(i).Name

		if vin.Field(i).Type().Kind() == reflect.Struct {
			// find the field which name equals 'fieldName',and it's kind is 'struct'
			field := rout.FieldByName(fieldName)
			if field.IsValid() && field.Type().Kind() == reflect.Struct {
				h.WraperLimiter(vin.Field(i).Interface(), field.Addr().Interface())
			} else {
				h.WraperLimiter(vin.Field(i).Interface(), out)
			}
			continue
		}

		field, exists := rout.Type().FieldByName(fieldName)

		if !exists || field.Type.Kind() != reflect.Func {
			continue
		}

		fn := vin.FieldByName(fieldName)
		if fn.IsNil() || fn.Kind() != reflect.Func {
			continue
		}

		rout.FieldByName(fieldName).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
			return h.CallProxy(fieldName, fn, args)
		}))
	}
}

// deprecated: todo: NEED A full test
func (h *RateLimiter) WrapFuncField(in interface{}, out interface{}) {

	vin := reflect.ValueOf(in)
	rout := reflect.ValueOf(out).Elem()

	for i := 0; i < vin.NumField(); i++ {
		method := vin.Type().Field(i).Name

		if vin.Field(i).Type().Kind() == reflect.Struct {
			h.WraperLimiter(vin.Field(i).Interface(), out)
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
			return h.CallProxy(method, fn, args)
		}))
	}
}

// deprecated: todo : NEED A full test
func (h *RateLimiter) WrapFunctions(in interface{}, out interface{}) {
	vin := reflect.ValueOf(in)
	vinType := reflect.TypeOf(in)

	vOut := reflect.ValueOf(out).Elem()

	for i := 0; i < vinType.NumMethod(); i++ {
		method := vinType.Method(i).Name

		field, exists := vOut.Type().FieldByName(method)

		if !exists || field.Type.Kind() != reflect.Func {
			continue
		}

		fn := vin.MethodByName(method)
		vOut.FieldByName(method).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
			return h.CallProxy(method, fn, args)
		}))
	}
}

func (h *RateLimiter) ProxyLimitFullAPI(in interface{}, out interface{}) {
	outs := GetInternalStructs(out)
	for _, out := range outs {
		rint := reflect.ValueOf(out).Elem()
		ra := reflect.ValueOf(in)

		for f := 0; f < rint.NumField(); f++ {
			field := rint.Type().Field(f)
			fn := ra.MethodByName(field.Name)

			rint.Field(f).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) (results []reflect.Value) {
				return h.CallProxy(field.Name, fn, args)
			}))
		}
	}
}
