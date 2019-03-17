package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/syhlion/greq"
	"github.com/syhlion/requestwork.v2"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var lua = `
function Hello()
local http = require("httpclient")
local client = http.client()

-- GET
local request = http.request("GET", "http://127.0.0.1:1919")
local result, err = client:do_request(request)
if err then error(err) end
if not(result.code == 200) then error("code") end
print(result.body)
end
`

type AddJobPayload struct {
	GroupName       string `json:"group_name"`
	Name            string `json:"name"`
	Exectime        int64  `json:"exec_time"`
	RequestUrl      string `json:"request_url"`
	IntervalPattern string `json:"interval_pattern"`
	ExecCommand     string `json:"exec_command"`
	Timeout         int64  `json:"timeout"`
	UseGroupOtp     bool   `json:"use_group_otp"`
}

func main() {
	work := requestwork.New(100)
	client := greq.New(work, 60*time.Second, true)
	scheduleTime := time.Now().Add(30 * time.Second).Unix()
	p := AddJobPayload{
		GroupName:       "YM55",
		Name:            "Hello",
		Exectime:        scheduleTime,
		IntervalPattern: "@once",
		RequestUrl:      "LUA@",
		ExecCommand:     lua,
		Timeout:         1,
	}
	b, err := json.Marshal(p)
	if err != nil {
		log.Println(err)
		return
	}
	for i := 0; i < 1500; i++ {
		bo := bytes.NewReader(b)
		d, _, err := client.PostRaw("http://127.0.0.1:7777/add", bo)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(string(d))
	}
	fmt.Println("SERVER START !!!!")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := time.Now().Unix() - scheduleTime
		if t > 1 {
			log.Println("timeout:", t)
		}
	})

	log.Fatal(http.ListenAndServe(":1919", nil))

}
