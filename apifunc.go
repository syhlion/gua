package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/syhlion/gua/luacore"
	"github.com/syhlion/gua/luaweb"
	lua "github.com/yuin/gopher-lua"
)

func LuaEntrance(rpool *redis.Pool, lpool *luacore.LStatePool) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
		l := lpool.Get()
		defer lpool.Put(l)
		l.SetContext(ctx)
		luaweb.Load(w, r, l, logger)
		var s = `
		function HelloWorld() 
			local json = require("json")
			local redis = require("redis")
			local conn,err = redis.open("127.0.0.1:6379",0)
			local reply,err = conn:exec("GET",{"YM"})
			if err then error(err) end

			local table = {
				["method"]=method(),
				["headers"]=headers(),
				["YM"]=tonumber(reply)
			}
			local result,err = json.encode(table)
			if err then error(err) end
			print(result)
		end
		`
		err := l.DoString(s)
		if err != nil {
			logger.Errorf("lua execute err:%#v\n", err)
			return
		}
		lfunc := l.GetGlobal("HelloWorld")
		switch r := lfunc.(type) {
		case *lua.LFunction:
			err := l.CallByParam(lua.P{
				Fn:      r,
				NRet:    1,
				Protect: true,
			})

			if err != nil {
				logger.Errorf("lua func exec err:%#v\n", err)
				return
			}
		default:
			logger.Errorf("lua not func \n")
			return
		}

	}
}
