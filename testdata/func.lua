function HelloWorld()
     local json = require("json")
     local redis = require("redis")
     local conn,err = redis.open("127.0.0.1:6379",0)
     local reply,err = conn:exec("GET",{"YM"})
     if err then error(err) end

     local table = {
         ["method"]=method(),
         ["headers"]=headers(),
         ["YM"]=tonumber(reply)
     }
     local result,err = json.encode(table)
     if err then error(err) end
     print(result)
end
