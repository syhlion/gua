package luacore

import (
	libs "github.com/syhlion/glua-libs"
	lua "github.com/yuin/gopher-lua"
)

func New() *LStatePool {
	return &LStatePool{
		make(chan *lua.LState, 1000),
	}
}

type LStatePool struct {
	saved chan *lua.LState
}

func (pl *LStatePool) Get() (l *lua.LState) {

	//因要提供給外部使用怕變數污染 取消pool
	/*
		select {
		case l = <-pl.saved:
		default:
			l = pl.new()
		}
		return
	*/
	l = pl.new()
	return
}

func (pl *LStatePool) new() (l *lua.LState) {
	l = lua.NewState()
	libs.Preload(l)
	return
}

func (pl *LStatePool) Put(L *lua.LState) {
	L.Close()
	/*
		select {
		case pl.saved <- L:
		default:
		}
	*/
}
