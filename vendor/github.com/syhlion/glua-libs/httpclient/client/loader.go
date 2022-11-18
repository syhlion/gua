//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/http
package http

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("http_client", Loader)
}

func Loader(L *lua.LState) int {

	http_client_ud := L.NewTypeMetatable(`http_client_ud`)
	L.SetGlobal(`http_client_ud`, http_client_ud)
	L.SetField(http_client_ud, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"do_request": DoRequest,
	}))

	http_request_ud := L.NewTypeMetatable(`http_request_ud`)
	L.SetGlobal(`http_request_ud`, http_request_ud)
	L.SetField(http_request_ud, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"set_basic_auth": SetBasicAuth,
		"header_set":     HeaderSet,
	}))

	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"client":  New,
	"request": NewRequest,
}
