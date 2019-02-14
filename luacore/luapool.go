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

	select {
	case l = <-pl.saved:
	default:
		l = pl.new()
	}
	return
}

func (pl *LStatePool) new() (l *lua.LState) {
	l = lua.NewState()
	l.PreloadModule("httpclient", CurlLoader)
	l.PreloadModule("mysql", MysqlLoader)
	libs.Preload(l)
	return
}

func (pl *LStatePool) Put(L *lua.LState) {
	select {
	case pl.saved <- L:
	default:
	}
}
