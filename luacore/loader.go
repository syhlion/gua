package luacore

import (
	"fmt"
	"io/ioutil"
	"net/http"

	lua "github.com/yuin/gopher-lua"
)

var curlExports = map[string]lua.LGFunction{
	"get": HttpGet,
}
var mysqlExports = map[string]lua.LGFunction{}

func HttpGet(l *lua.LState) int {
	url := string(l.ToString(1))
	resp, err := http.Get(url)
	if err != nil {
		l.Push(lua.LNil)
		l.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.Push(lua.LNil)
		l.Push(lua.LString(err.Error()))
		return 2
	}
	fmt.Println(string(b))
	l.Push(lua.LString(string(b)))
	l.Push(lua.LNil)
	return 2
}
func HttpPost(l *lua.LState) int {
	return 2
}

func CurlLoader(l *lua.LState) int {
	mod := l.SetFuncs(l.NewTable(), curlExports)
	l.Push(mod)
	return 1
}
func MysqlLoader(l *lua.LState) int {
	mod := l.SetFuncs(l.NewTable(), mysqlExports)
	l.Push(mod)
	return 1
}
