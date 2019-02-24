function Hello()
local http = require("http")
local client = http.client()

-- GET
local request = http.request("GET", "http://127.0.0.1:9999")
local result, err = client:do_request(request)
if err then error(err) end
if not(result.code == 200) then error("code") end
print(result.body)
end
