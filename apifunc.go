package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/pquerna/otp/totp"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/luacore"
	"github.com/syhlion/gua/luaweb"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
	lua "github.com/yuin/gopher-lua"
)

func LuaEntrance(quene delayquene.Quene, apiRedis *redis.Pool, lpool *luacore.LStatePool) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		funcName := vars["func_name"]
		query := r.URL.Query()
		otpCode := query.Get("otp_code")
		if groupName == "" || funcName == "" {
			err := errors.New("no group_name || func_name")
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		groupOtp, err := quene.GroupInfo(groupName)
		if err != nil {
			log.Printf("Error no group: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		funcKey := fmt.Sprintf("FUNC-%s-%s", groupName, funcName)
		conn := apiRedis.Get()
		defer conn.Close()
		b, err := redis.Bytes(conn.Do("GET", funcKey))
		if err != nil {
			log.Printf("Error no api func: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		apiFunc := &guaproto.Func{}
		err = proto.Unmarshal(b, apiFunc)
		if err != nil {
			log.Printf("Error no group: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}

		if apiFunc.UseOtp {
			if apiFunc.DisableGroupOtp {
				if !totp.Validate(otpCode, apiFunc.OtpToken) {
					err = errors.New("otp error")
					restresp.Write(w, err, http.StatusBadRequest)
					return
				}
			} else {
				if !totp.Validate(otpCode, groupOtp) {
					err = errors.New("group otp error")
					restresp.Write(w, err, http.StatusBadRequest)
					return
				}

			}
		}

		ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
		l := lpool.Get()
		defer lpool.Put(l)
		l.SetContext(ctx)
		luaweb.Load(w, r, l, logger)
		err = l.DoString(string(apiFunc.LuaBody))
		if err != nil {
			logger.Errorf("lua execute err:%#v\n", err)
			return
		}
		lfunc := l.GetGlobal(apiFunc.Name)
		switch r := lfunc.(type) {
		case *lua.LFunction:
			err := l.CallByParam(lua.P{
				Fn:      r,
				NRet:    1,
				Protect: true,
			})

			if err != nil {
				logger.Errorf("lua func exec err:%#v\n", err)
				return
			}
		default:
			logger.Errorf("lua not func %#v \n", r)
			return
		}

	}
}
