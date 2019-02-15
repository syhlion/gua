package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/syhlion/gua/delayquene"
)

func main() {
	config := delayquene.Config{
		RedisForReadyAddr:         "127.0.0.1:6379",
		RedisForReadyDBNo:         0,
		RedisForReadyMaxIdle:      10,
		RedisForReadyMaxConn:      100,
		RedisForDelayQueneAddr:    "127.0.0.1:6379",
		RedisForDelayQueneDBNo:    0,
		RedisForDelayQueneMaxIdle: 10,
		RedisForDelayQueneMaxConn: 100,
	}
	quene, err := delayquene.New(config)
	if err != nil {
		fmt.Println(err)
		return
	}
	var luaBody = `
	
		function Count()
			local client = require("httpclient")
			resp,err = client.get("http://127.0.0.1:6666/?lua=iamking")
			if err ~= "" then 
				return
			end
	   end
	
	`
	http.HandleFunc("/normal", func(w http.ResponseWriter, r *http.Request) {
		job := &delayquene.Job{
			Name:            "Count",
			Id:              quene.GenerateId(),
			Exectime:        time.Now().Add(5 * time.Second).Unix(),
			IntervalPattern: "@every 1s",
			RequestUrl:      "http://127.0.0.1:6666/?echo=5",
			//LuaBody: luaBody,
		}
		err = quene.Push(job)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("normal")
	})
	http.HandleFunc("/lua", func(w http.ResponseWriter, r *http.Request) {
		job := &delayquene.Job{
			Name:            "Count",
			Id:              quene.GenerateId(),
			Exectime:        time.Now().Add(5 * time.Second).Unix(),
			IntervalPattern: "@every 1s",
			//RequestUrl: "http://127.0.0.1:6666",
			LuaBody: []byte(luaBody),
		}
		err = quene.Push(job)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("lua")
	})

	log.Fatal(http.ListenAndServe(":8888", nil))

}
