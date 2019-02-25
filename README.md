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


### add

`POST /add`

Body:

[http.json](./testdata/http.json)

```
{
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
$ curl "@http.json" http://{{yourhost}}/add
```


