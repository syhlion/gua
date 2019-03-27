package telegram

import (
	lua "github.com/yuin/gopher-lua"
)

// Preload adds db to the given Lua state's package.preload table. After it
// has been preloaded, it can be loaded using require:
//
//  local db = require("db")
func Preload(L *lua.LState) {
	L.PreloadModule("telegram", Loader)
}

// Loader is the module loader function.
func Loader(L *lua.LState) int {

	db_ud := L.NewTypeMetatable(`telegram_ud`)
	L.SetGlobal(`telegram_ud`, db_ud)
	L.SetField(db_ud, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"send": Send,
	}))

	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"send": New,
}
