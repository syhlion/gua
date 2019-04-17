//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/crypto
package crypto

import (
	lua "github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("crypto", Loader)
}

func Loader(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

var api = map[string]lua.LGFunction{
	"md5":    MD5,
	"sha256": SHA256,
}
