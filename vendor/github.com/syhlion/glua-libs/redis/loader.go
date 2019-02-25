package redis

import (
	lua "github.com/yuin/gopher-lua"
)

// Preload adds db to the given Lua state's package.preload table. After it
// has been preloaded, it can be loaded using require:
//
//  local db = require("db")
func Preload(L *lua.LState) {
	L.PreloadModule("redis", Loader)
}

// Loader is the module loader function.
func Loader(L *lua.LState) int {

	db_ud := L.NewTypeMetatable(`redis_ud`)
	L.SetGlobal(`redis_ud`, db_ud)
	L.SetField(db_ud, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"exec":  Do,
		"close": Close,
	}))

	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"open": Open,
}
