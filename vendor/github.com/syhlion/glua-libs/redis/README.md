# redis

## Usage

```lua
local redis = require("redis")

local conn, err = redis.open("127.0.0.1:6379",0)
if err then error(err) end


local reply, err = conn:exec("SET",{"KKK","abcdef"})
if err then error(err) end

print(reply)
local reply, err = conn:exec("ZADD",{"page_rank","10","google.com"})
if err then error(err) end
local reply, err = conn:exec("ZADD",{"page_rank","11","yahoo.com"})
if err then error(err) end

local replys, err = conn:exec("ZRANGE",{"page_rank",0,-1,"WITHSCORES"})
if err then error(err) end

for i, v in pairs(replys) do
    print(i,v)
end


conn:close()
```

