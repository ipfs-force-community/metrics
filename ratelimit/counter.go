package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"
)

type ReqCount struct {
	Ts, Count int64
}

func (rc *ReqCount) Time() time.Time {
	return time.Unix(rc.Ts, 0)
}

func toRedisKey(user, service, method string) string {
	return fmt.Sprintf("statictis.%s.%s.%s", user, service, method)
}

var luaScriptIncrReqCount = redis.NewScript(`
redis.replicate_commands()
	
local key = KEYS[1]

local interval = tonumber(ARGV[1])
local keep = tonumber(ARGV[2])

local now = tonumber(redis.call("TIME")[1])
local size = tonumber(redis.call("llen", key))

if size == 0 then
	req_count_json = cjson.encode({Ts=now, Count=1})
	redis.call("rpush", key, req_count_json)
	return req_count_json
end	

if (size >= keep) then
	redis.call("ltrim", key, size-keep+1, -1)
end
	
local req_count_json = redis.call("rpop", key)
local req_count = cjson.decode(req_count_json)
local count = tonumber(req_count["Count"])
local at = req_count["Ts"]	

if ((now - at) < interval) then
	req_count["Count"] = count + 1	
	req_count_json = cjson.encode(req_count)
	redis.call("rpush", key, req_count_json)
	return req_count_json
end
	
local req_count_json_now = cjson.encode({Ts=now, Count=count+1})
redis.call("rpush", key, req_count_json, req_count_json_now)
return {req_count_json, req_count_json_now}
`)

// interval:统计的时间间隔(秒)
// maxSize: list的大小.
// maxSize * interval =  最大保存多少时间的请求数量统计
func (authMux *RateLimiter) incrReqCount(user, service, method string,
	interval, maxSize int64) (*ReqCount, error) {
	client := authMux.rdb
	key := toRedisKey(user, service, method)

	cmd := luaScriptIncrReqCount.Run(context.TODO(), client, []string{key}, interval, maxSize)
	if err := cmd.Err(); err != nil {
		return nil, cmd.Err()
	}
	var reqCount ReqCount
	return &reqCount, json.Unmarshal([]byte(cmd.String()), &reqCount)
}
