# mysql

origin code: https://github.com/vadv/gopher-lua-libs/tree/master/db

## Usage

```lua
local db = require("db")

local mysql, err = db.open("mysql", "root:1234@(localhost:3307)/pitaya_game?charset=utf8&parseTime=True&loc=Local")
if err then error(err) end


local _, err = mysql:exec("CREATE TABLE t (id int, name string);")
if err then error(err) end

for i = 1, 10 do
    local query = "INSERT INTO t VALUES ("..i..", \"name-"..i.."\");"
    if i % 2 == 0 then query = "INSERT INTO t VALUES ("..i..", NULL);" end
    local _, err = mysql:exec(query)
    if err then error(err) end
end

local result, err = mysql:query("select * from t;")
if err then error(err) end

for i, v in pairs(result.columns) do
    if i == 1 then if not(v == "id") then error("error") end end
    if i == 2 then if not(v == "name") then error("error") end end
end

for _, row in pairs(result.rows) do
    for id, name in pairs(result.columns) do
        print(name, row[id])
    end
end

local err = mysql:close()
if err then error(err) end
```

