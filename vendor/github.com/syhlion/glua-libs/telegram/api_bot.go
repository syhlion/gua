package telegram

import (
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type luaTgBot struct {
	sync.Mutex
	bot *tgbotapi.BotAPI
}

func (l *luaTgBot) constructor(token string) (b lTgBot, err error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return
	}
	result := &luaTgBot{}
	result.bot = bot
	return result, nil
}

func (l *luaTgBot) getBot() *tgbotapi.BotAPI {
	l.Lock()
	defer l.Unlock()
	return l.bot
}
