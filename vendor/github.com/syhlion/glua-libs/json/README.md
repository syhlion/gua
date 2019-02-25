# json

 origin code: https://github.com/vadv/gopher-lua-libs/tree/master/json

## Usage

```lua
local json = require("json")

-- json.encode()
local jsonString = [[
    {
        "a": {"b":1}
    }
]]
local result, err = json.decode(jsonString)
if err then error(err) end

-- json.decode()
local table = {a={b=1}}
local result, err = json.encode(table)
if err then error(err) end
```
