//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/json
package json

import (
	lua "github.com/yuin/gopher-lua"
)

func Decode(L *lua.LState) int {
	str := L.CheckString(1)

	value, err := ValueDecode(L, []byte(str))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(value)
	return 1
}

func Encode(L *lua.LState) int {
	value := L.CheckAny(1)

	data, err := ValueEncode(value)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(string(data)))
	return 1
}
