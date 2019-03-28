# gua

Support crontab style Schedule System  implement by golang

## Changelog

[CHANGELOG](./CHANGELOG.md)

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



## QuickStart:

[docker-compose](./docker-compose)





## Time Trigger Architecture


![time-trigger-architecture](./other/gua-arch2-timetrigger.png)


## Api Trigger Architecture


![time-trigger-architecture](./other/gua-arch2-apitrigger.png)




## Api



### admin api

[api v1](./apiv1.md)


## How to use time trigger for lua mode?

[lua mode doc](./luamode.md)


## How to use api trigger

[api trigger](./funclua.md)

