//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/strings
package strings

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("strings", Loader)
}

func Loader(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"split":       Split,
	"trim":        Trim,
	"trim_prefix": TrimPrefix,
	"trim_suffix": TrimSuffix,
	"has_prefix":  HasPrefix,
	"has_suffix":  HasSuffix,
	"contains":    Contains,
}
