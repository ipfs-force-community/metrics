package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"
)

const (
	FAILED = iota
	SUCC
)

var rds *redis.Client

var initFunnelScript = `
	-- 分别初始化漏斗结构的4个字段capacity、left_quota、leaking_rate、leaking_time
	-- capacity:漏斗容量
	-- left_quota:漏斗剩余空间
	-- leaking_rate:漏嘴流水速率
	-- leaking_time:上一次漏水时间
	local key
	for i,j in ipairs(ARGV) 
	do if i%2 == 0
		then
			redis.pcall('hsetnx', KEYS[1], key, j)
		else
			key = j
		end
	end`

var initFunnelSha string

var makeSpaceScript = `
	local leaking_time = tonumber(redis.pcall('hget', KEYS[1], 'leaking_time'))
	local leaking_rate = tonumber(redis.pcall('hget', KEYS[1], 'leaking_rate'))
	local left_quota = tonumber(redis.pcall('hget', KEYS[1], 'left_quota'))
	local capacity = tonumber(redis.pcall('hget', KEYS[1], 'capacity'))
	local now = tonumber(ARGV[1])
	local delta_time = now - leaking_time -- 距离上一次漏水过去了多久
	local delta_quota = leaking_rate * delta_time -- 又可以腾出不少空间了
	
	redis.pcall('hset', KEYS[1], 'leaking_time', now) -- 记录漏水时间
	if delta_quota + left_quota >= capacity then -- 剩余空间不得高于容量
		redis.pcall('hset', KEYS[1], 'left_quota', capacity) 
	else 
		redis.pcall('hset', KEYS[1], 'left_quota', delta_quota + left_quota) -- 增加剩余空间
	end
`
var makeSpaceSha string

var wateringScript = `
	local left_quota = tonumber(redis.pcall('hget', KEYS[1], 'left_quota'))
	local quota = tonumber(ARGV[1])
	if left_quota >= quota then -- 判断剩余空间是否足够
		redis.pcall('hset', KEYS[1], 'left_quota', left_quota-quota) 
		return 1
	else
		return 0
	end
`

var wateringSha string

func init() {
	initRedisClient()
}

func initRedisClient() {
	rds = redis.NewClient(&redis.Options{
		Addr:     "192.168.1.125:6379",
		Password: "",
		DB:       0,
	})

	var err error
	var ctx = context.TODO()
	initFunnelSha, err = rds.ScriptLoad(ctx, initFunnelScript).Result()
	if err != nil {
		panic(err)
	}

	makeSpaceSha, err = rds.ScriptLoad(ctx, makeSpaceScript).Result()
	if err != nil {
		panic(err)
	}

	wateringSha, err = rds.ScriptLoad(ctx, wateringScript).Result()
	if err != nil {
		panic(err)
	}
}

func MakeSpace(key string) {
	now := time.Now().Unix()
	rds.EvalSha(context.TODO(), makeSpaceSha, []string{key}, now).Result()
}

// quota为每次处理请求所需要的资源配额
func Watering(key string, quota float64) bool {
	MakeSpace(key)
	ctx := context.TODO()
	res, err := rds.EvalSha(ctx, wateringSha, []string{key}, quota).Result()
	if err != nil {
		panic(err)
	}
	return res.(int64) == SUCC
}

func IsActionAllowed(uid, action string, capacity float64, leakingRate float64) bool {
	key := fmt.Sprintf("%v_%v", uid, action)
	rds.EvalSha(context.TODO(), initFunnelSha, []string{key}, "capacity", capacity, "left_quota", capacity, "leaking_rate", leakingRate, "leaking_time", time.Now().Unix())
	return Watering(key, 1)
}

func main() {
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
	var exit = make(chan interface{})
	go func() {
		for i := 0; i < 15; i++ {
			if val, err := luaScriptIncrReqCount.Run(context.TODO(), rds, []string{"zl"}, 2, 5).Result(); err != nil {
				fmt.Println(err.Error())
				return
			} else {
				fmt.Println(val)
			}
			time.Sleep(time.Second * 3)
			exit <- 0
		}
	}()

	cmd := rds.LPush(context.TODO(), "zl", `{"Count":0, "Ts":1627554920}`)
	if cmd.Err() != nil {
		return
	}

	if err := cmd.Err(); err != nil {
		fmt.Println(err.Error())
	}
	for i := 0; i < 20; i++ {
		fmt.Printf("%+v\n", IsActionAllowed("berryjam", "reply", 15, 0.5))
	}

	<-exit
}
