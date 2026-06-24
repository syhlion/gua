[unrelease]

[v2.0.0] — harden (breaking)

[Changed]

* delivery reduced to two transports: HTTP and gRPC. Removed REMOTE (remote-shell
  / gua-node) and LUA (in-process script) trigger types.
* HTTP callback is now a JSON POST envelope (was a GET ping); same fields as the
  gRPC trigger. `exec_command` -> `payload` (string).
* new gRPC services: `GuaAdmin` (CRUD, mirrors the REST API) and `GuaCallback`
  (`OnJobTrigger`, Push). Removed old node/JobReply RPCs.
* removed app-level OTP auth; group is now a pure namespace. Use transport-level
  auth (mTLS / gateway) if needed.
* HTTP client swapped greq/requestwork (archived) -> resty.

[Added]

* monitoring: `GET /v1/status`, `GET /v1/{group}/history`, web console `GET /ui`.
* execution history persisted to Redis with TTL retention (`GUA_HISTORY_TTL`).
* test suite: cron parser units + miniredis lifecycle integration + lock/fencing
  + stress/timing harness.
* docs refresh: ARCHITECTURE.md, MONITORING.md, EVAL.md, new drawio diagrams.

[Fixed / hardened]

* `STARTLOCK`/`JOBCHECKLOCK` are TTL + owner-token locks (Lua CAS release) — no
  more deadlock on holder crash; JobCheck no longer runs unlocked.
* per-firing de-dup fence on the ready queue (reclaim / JobCheck can't
  double-enqueue).
* SERVER-N slot fencing (owner token + heartbeat CAS) closes the snowflake
  node-id collision window; gRPC delivery connection pooling.
* removed panics (nil request_url), context leaks, the uncatchable SIGKILL
  handler, and the heavy lua/mysql/telegram dependency tree.

[v1.4.1]

* remove keys use and use scan iter

[v1.4.0]

* add job key JOB-{}-{}-scan life-time
* add auto check miss job key
* fix server recover

[v1.3.0]

* add remote group api

[v1.2.0]

* add delete jobs by group and job name

[v1.1.0]

[add]

* add GroupJobClear api

[v1.0.0]

[fix]

* fix http respone
* fix logger panic

[v1.0.0-rc6]

[fix]

* fix delpy func
* fix time start

[v1.0.0-rc5]

[fix]

* fix redis disconnect


[v1.0.0-rc4]

[Add]

* redis heartbeat 1 op/second


[v1.0.0-rc3]

[Fix]

* gua-node provide fault tolerance (300s)

[Add]

* gua add edit api (only edit request_url & exec_cmd)


[v1.0.0-rc2]

[Fix]

* gua grcp remote to gua-node forget close connection
* init lock bug


[v1.0.0-rc1]

[Add]

* Add License
* Add mux handler CORS 
* Add job & func filed memo

[Fix]

* time trigger lua param add groupname
* use mulit build stage for docker
* fix api md
* fix admin namepsace to loghook namespace
* vendor remake


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
