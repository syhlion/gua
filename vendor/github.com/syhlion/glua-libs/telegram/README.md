# telegram

## Usage

```lua

local telegram = require("telegram")

local token=""
local chatId = 0
local msg ="helloworld"

bot , err := telegram.new(token)
if err then error(err) end

reply,err = bot.send(chatId,chatMsg)
if err then error(err) end

```


## How to generate your token


[telegram api](https://core.telegram.org/bots)

## How to get self chat Id

open your browser:

` https://api.telegram.org/bot{token}/getUpdates`

