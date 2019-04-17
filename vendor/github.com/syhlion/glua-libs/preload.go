package libs

import (
	crypto "github.com/syhlion/glua-libs/crypto"
	httpclient "github.com/syhlion/glua-libs/httpclient"
	json "github.com/syhlion/glua-libs/json"
	db "github.com/syhlion/glua-libs/mysql"
	redis "github.com/syhlion/glua-libs/redis"
	regexp "github.com/syhlion/glua-libs/regexp"
	strings "github.com/syhlion/glua-libs/strings"
	telegram "github.com/syhlion/glua-libs/telegram"
	time "github.com/syhlion/glua-libs/time"

	lua "github.com/yuin/gopher-lua"
)

// Preload(): preload all gopher lua packages
func Preload(L *lua.LState) {
	time.Preload(L)
	strings.Preload(L)
	httpclient.Preload(L)
	regexp.Preload(L)
	json.Preload(L)
	db.Preload(L)
	crypto.Preload(L)
	redis.Preload(L)
	telegram.Preload(L)
}
