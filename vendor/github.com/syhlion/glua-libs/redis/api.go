package redis

import (
	"github.com/gomodule/redigo/redis"
	lua "github.com/yuin/gopher-lua"
)

const (
	// max idle connections
	MaxIdleConns = 1
	// max open connections
	MaxOpenConns = 1
)

type lRedis interface {
	constructor(string, int) (lRedis, error)
	getPool() *redis.Pool
}

var redisDB = new(luaRedis)

func Open(L *lua.LState) int {

	conn := L.CheckString(1)
	db := L.CheckInt(2)
	result, err := redisDB.constructor(conn, db)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	ud := L.NewUserData()
	ud.Value = result
	L.SetMetatable(ud, L.GetTypeMetatable(`redis_ud`))
	L.Push(ud)
	return 1
}
func checkRedis(L *lua.LState, n int) lRedis {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(lRedis); ok {
		return v
	}
	L.ArgError(n, "redis expected")
	return nil
}

func Do(L *lua.LState) int {
	dbInterface := checkRedis(L, 1)
	cmd := L.CheckString(2)
	arg := L.CheckTable(3)
	redisArg := make([]interface{}, 0)
	arg.ForEach(func(k lua.LValue, v lua.LValue) {
		switch v.(type) {
		case lua.LString:
			redisArg = append(redisArg, string(v.(lua.LString)))
		case lua.LNumber:
			redisArg = append(redisArg, float64(v.(lua.LNumber)))
		}
	})

	redisPool := dbInterface.getPool()
	conn := redisPool.Get()
	defer conn.Close()
	reply, err := conn.Do(cmd, redisArg...)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	var rows *lua.LTable
	if v, ok := reply.([]interface{}); ok {
		rows, err = parseReplys(v, L)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(rows)
	} else {
		lv, err := parseReply(reply, L)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lv)

	}
	return 1
}

func Close(L *lua.LState) int {
	dbInterface := checkRedis(L, 1)
	pool := dbInterface.getPool()
	pool.Close()
	return 0
}
