# Api 

`GET /version`

Example:

```
$ curl http://{{youhost}}/version
```


# Api v1


### Register Group

`POST /v1/register/group`

Body:

[register_group.json](./testdata/register_group.json)

```
{
  "group_name": "YM55", //rule: [a-zA-Z0-9_]{1,22}
}
```

Example:

```
$ curl -d "@register_group.json" -X POST http://{{yourhost}}/v1/register/group
```

### Remove Group

`POST /v1/remove/group`

Body:

[remove_group.json](./testdata/remove_group.json)

```
{
  "group_name": "YM55", 
}
```

Example:

```
$ curl -d "@remove_group.json" -X POST http://{{yourhost}}/v1/remove/group
```


### Add Job

`POST /v1/add/job`

Body:

[http.json](./testdata/http.json)
[httpbyjobid.json](./testdata/httpbyjobid.json)
[httponce.json](./testdata/httponce.json)

```
{
  "group_name": "YM55", //required
  "name": "Hello",  //required
  "job_id":"1234ABC", //option  you can specify your job_id rule: [a-zA-Z0-9_]{1,22}
  "exec_time": 0, //unixtime. if exec_time == 0 exec now
  "request_url": "HTTP@http://127.0.0.1:9999?test=yao", //required prefix REMOTE|LUA|HTTP
  "interval_pattern": "@once", //use crontab schema or @once or @every 5s
  "exec_command": "",
  "timeout": 1,
  "memo":""
}
```

### Edit Job

`POST /v1/edit/job`

Body:

```
{
  "group_name": "YM55",
  "job_id" :"1107521665400573952",
  "request_url": "HTTP@http://127.0.0.1:9999?test=yao", //prefix REMOTE|LUA|HTTP
  "exec_command": ""
}
```



### Pause Job

`POST /v1/pause/job`

Body:

[pause.json](./testdata/pause.json)

```
{
    "group_name":"YM55",
    "job_id":"1107521665400573952"
}
```

Example:

```
$ curl -d "@pause.json" -X POST http://{{yourhost}}/v1/pause/job
```


### Active Job

`POST /v1/active/job`

Body:

[active.json](./testdata/active.json)

```
{
    "group_name":"YM55",
    "job_id":"1107521665400573952",
    "exec_time":0 //unixtime. if exec_time == 0 exec now

}
```

Example:

```
$ curl -d "@active.json" -X POST http://{{yourhost}}/v1/active/job
```


### Delete Job

`POST /v1/delete/job`

Body:

[delete.json](./testdata/delete.json)

```
{
    "group_name":"YM55",
    "job_id":"1107521665400573952"
}
```

Example:

```
$ curl -d "@delete.json" -X POST http://{{yourhost}}/v1/delete/job
```


### Get job list

`GET /v1/{group_name}/job/list`


Example:

```
$ curl http://{{yourhost}}/v1/{{your_group_name}}/job/list
```


### Get group list

`GET /v1/group/list`


Example:

```
$ curl http://{{yourhost}}/v1/group/list
```


### Get group info

`GET /{group_name}/v1/group/info`


Example:

```
$ curl http://{{yourhost}}/{group_name}/v1/group/info
```


### Get node list

`GET /v1/{group_name}/node/list`


Example:

```
$ curl http://{{yourhost}}/{group_name}/v1/node/list
```


### Add Func

`POST /v1/add/func`

Body:

[http.json](./testdata/addfunc.json)

```
{
  "group_name": "YM55",
  "name": "Hello",
  "use_otp": false,
  "disable_group_otp": false,
  "lua_body": "function HelloWorld()\n     local json = require(\"json\")\n     local redis = require(\"redis\")\n     local conn,err = redis.open(\"127.0.0.1:6379\",0)\n     local reply,err = conn:exec(\"GET\",{\"YM\"})\n     if err then error(err) end\n\n     local table = {\n         [\"method\"]=method(),\n         [\"headers\"]=headers(),\n         [\"YM\"]=tonumber(reply)\n     }\n     local result,err = json.encode(table)\n     if err then error(err) end\n     print(result)\nend",
  "momo":"",
}
```

Example:

```
$ curl -d "@addfunc.json" -X POST http://{{yourhost}}/v1/add/func
```

> You can request "http://{{yourhost}}:{{your HTTP_FUNC_LISTEN}}/YM55/Hello" 

### clear job by group

`DELETE /v1/{group_name}/job/clear`

Example:

```
curl --request DELETE 'http://{{yourhost}}/v1/{{your_group_name}}/job/clear'
```

### delete jobs by group and job name

`DELETE /v1/{group_name}/job/delete/{job_name}`

Example:

``` curl
curl --request DELETE 'http://{{yourhost}}/v1/{{your_group_name}}/job/delete/{{job_name}}'
```


### Dump all data

`GET /v1/dump/all`


Example:

```
$ wget http://{{yourhost}}/v1/dump/all
```

### Dump by group

`GET /v1/{group_name}/dump`


Example:

```
$ wget http://{{yourhost}}/v1/{{group_name}}/dump
```

### Import data

`POST /v1/import`

Example:

```
$ curl --data-binary "@{{file_name}}.tar.gz" -X POST http://{{yourhost}}/import
```


## cron style pattern


interval_pattern support scheam


### cron format

Field name   | Mandatory? | Allowed values  | Allowed special characters
----------   | ---------- | --------------  | --------------------------
Seconds      | Yes        | 0-59            | * / , -
Minutes      | Yes        | 0-59            | * / , -
Hours        | Yes        | 0-23            | * / , -
Day of month | Yes        | 1-31            | * / , - ?
Month        | Yes        | 1-12 or JAN-DEC | * / , -
Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?


### special schedules

Entry                  | Description                                | Equivalent To
-----                  | -----------                                | -------------
@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 0 1 1 *
@monthly               | Run once a month, midnight, first of month | 0 0 0 1 * *
@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 0 * * 0
@daily (or @midnight)  | Run once a day, midnight                   | 0 0 0 * * *
@hourly                | Run once an hour, beginning of hour        | 0 0 * * * *

### interval

`@every <duration>`

example:

`@every 5s`


