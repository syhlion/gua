//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/time
package time

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("time", Loader)
}

func Loader(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"unix":      Unix,
	"unix_nano": UnixNano,
	"sleep":     Sleep,
	"parse":     Parse,
	"format":    Format,
}
