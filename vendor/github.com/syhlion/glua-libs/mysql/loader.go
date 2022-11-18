//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/db
package db

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("db", Loader)
}

func Loader(L *lua.LState) int {

	db_ud := L.NewTypeMetatable(`db_ud`)
	L.SetGlobal(`db_ud`, db_ud)
	L.SetField(db_ud, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"query": Query,
		"exec":  Exec,
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
