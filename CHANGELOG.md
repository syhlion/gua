[unrelease]


[Add]

* Add License
* Add mux handler CORS 

[Fix]

* time trigger lua param add groupname
* use mulit build stage for docker


[Add]

* lua add telegram api



[v1.0.0.beta5] 2019-03-27

[Add]

* add dump all data & dump by group
* add import data
* add global version api


[Fix]

* fix api prefix path /v1



[v1.0.0.beta4] 2019-03-22

[Add]

* add get group list api
* add get group info api
* add get node list api

[Fix]

* fix get jobs  exec_cmd byte to string
* fix get job list api for restful old: /jobs    new: /{group_name}/job/list
* fix register api for restful old: /register    new: /register/group
* fix add job list api for restful old: /add    new: /add/job
* fix active job list api for restful old: /active    new: /active/job
* fix delete job list api for restful old: /delete    new: /delete/job
* fix pause job list api for restful old: /pause    new: /pause/job





[v1.0.0.beta3] 2019-03-19

[Fix]

* fix env default

[Add]

* add remote exec log callback
* add query nodes api



[v1.0.0.beta2] 2019-03-18

[Add]
* add group api
* add luaweb core
* add func api
* add query api


[Remove]
* remove luacore loader.go

[Fix]
* fix readme
* fix delayquene BLPOP use c.Do
* fix glua redis lib bug

[v1.0.0.beta1] 2019-02-25

[Add]

* add readme
* add gitlab ci
* add testdata
* add makefile
