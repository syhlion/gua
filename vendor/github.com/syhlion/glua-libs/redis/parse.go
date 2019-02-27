package redis

import (
	"log"

	lua "github.com/yuin/gopher-lua"
)

func parseReply(reply interface{}, L *lua.LState) (lv lua.LValue, err error) {

	switch vv := reply.(type) {
	case []byte:
		lv = lua.LString(vv)
	default:
		log.Printf("[ERROR] unknown type (value: `%#v`, converted: `%#v`)\n", reply, vv)
		lv = lua.LNil
	}

	return
}
func parseReplys(replys []interface{}, L *lua.LState) (*lua.LTable, error) {

	rows := L.CreateTable(len(replys), 1)
	for _, v := range replys {

		switch vv := v.(type) {
		case []byte:
			rows.Append(lua.LString(string(v.([]byte))))
		default:
			log.Printf("[ERROR] unknown replys type (value: `%#v`, converted: `%#v`)\n", v, vv)
			rows.Append(lua.LNil)
		}
	}

	return rows, nil
}
