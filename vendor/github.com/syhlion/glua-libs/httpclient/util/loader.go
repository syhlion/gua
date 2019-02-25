//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/http
package util

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("http_util", Loader)
}

func Loader(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"query_escape":   QueryEscape,
	"query_unescape": QueryUnescape,
}
