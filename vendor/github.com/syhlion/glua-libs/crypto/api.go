//origin code: https://github.com/vadv/gopher-lua-libs/tree/master/crypto
package crypto

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func MD5(L *lua.LState) int {
	str := L.CheckString(1)
	hash := md5.Sum([]byte(str))
	L.Push(lua.LString(fmt.Sprintf("%x", hash)))
	return 1
}

func SHA256(L *lua.LState) int {
	str := L.CheckString(1)
	hash := sha256.Sum256([]byte(str))
	L.Push(lua.LString(fmt.Sprintf("%x", hash)))
	return 1
}
