# gua

## Changelog

[CHANGELOG](./CHANGELOG)

## Usage


use env file

```
$ ./gua start -e {{env.example}} 
```


or has env

```
$ ./gua start
```

[env.example](./env.example)



## api

### register group

`POST /register`

Body:

[register_group.json](./testdata/register_group.json)

```
{
    "group_name":"YM55"
}
```

Example:

```
$ curl -d "@register_group.json" -X POST http://{{yourhost}}/register
```



### add

`POST /add`

Body:

[http.json](./testdata/http.json)

```
{
  "group_name": "YM55",
  "name": "Hello",
  "exec_time": 0, //unixtime. if exec_time == 0 exec now
  "request_url": "HTTP@http://127.0.0.1:9999?test=yao", //prefix REMOTE|LUA|HTTP
  "interval_pattern": "@once", //use crontab schema or @once or @every 5s
  "exec_command": "",
  "timeout": 1
}
```

Example:

```
$ curl -d "@http.json" -X POST http://{{yourhost}}/add
```

### pause

`POST /pause`

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
$ curl -d "@pause.json" -X POST http://{{yourhost}}/pause
```


### active

`POST /active`

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
$ curl -d "@active.json" -X POST http://{{yourhost}}/active
```


### delete

`POST /delete`

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
$ curl -d "@delete.json" -X POST http://{{yourhost}}/delete
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


