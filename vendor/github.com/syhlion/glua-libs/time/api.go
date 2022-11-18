//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/time
package time

import (
	"time"

	lua "github.com/yuin/gopher-lua"
)

func Unix(L *lua.LState) int {
	now := float64(time.Now().UnixNano()) / float64(time.Second)
	L.Push(lua.LNumber(now))
	return 1
}

func UnixNano(L *lua.LState) int {
	L.Push(lua.LNumber(time.Now().UnixNano()))
	return 1
}

func Sleep(L *lua.LState) int {
	val := L.CheckNumber(1)
	time.Sleep(time.Duration(val) * time.Second)
	return 0
}

func Parse(L *lua.LState) int {
	layout, value := L.CheckString(2), L.CheckString(1)
	result, err := time.Parse(layout, value)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	resultFloat := float64(result.UTC().UnixNano()) / float64(time.Second)
	L.Push(lua.LNumber(resultFloat))
	return 1
}

func Format(L *lua.LState) int {
	tt := float64(L.CheckNumber(1))
	sec := int64(tt)
	nsec := int64((tt - float64(sec)) * 1000000000)
	result := time.Unix(sec, nsec)
	layout := "Mon Jan 2 15:04:05 -0700 MST 2006"
	if L.GetTop() > 1 {
		layout = L.CheckString(2)
	}
	if L.GetTop() < 3 {
		L.Push(lua.LString(result.Format(layout)))
		return 1
	}
	location := L.CheckString(3)
	loc, err := time.LoadLocation(location)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	result = result.In(loc)
	L.Push(lua.LString(result.Format(layout)))
	return 1
}
