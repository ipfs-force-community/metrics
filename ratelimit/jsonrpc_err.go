package ratelimit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type respError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *respError) Error() string {
	if e.Code >= -32768 && e.Code <= -32000 {
		return fmt.Sprintf("RPC error (%d): %s", e.Code, e.Message)
	}
	return e.Message
}

type response struct {
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	ID      int64       `json:"id"`
	Error   *respError  `json:"error,omitempty"`
}

func rpcError(w http.ResponseWriter, user, host string, cap, used int64, recoverDur time.Duration) error {
	resp := response{
		Jsonrpc: "2.0",
		Error: &respError{
			Message: fmt.Sprintf("user(%s, %s), request is limted, cap:%d, used:%d, will reset in :%.2f(m)",
				user, host, cap, used, recoverDur.Minutes()),
		},
	}
	return json.NewEncoder(w).Encode(resp)
}
