package bot

import (
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

func Start() error {
	pref := telebot.Settings{
		Token:  "7656241581:AAHo0Dt2RWKw93uYQNV4riWBaDRgfGR8ayw",
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return err
	}

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("Добро пожаловать! Вы можете запросить конфигурацию для VPN.")
	})

	log.Println("Bot is running...")
	b.Start()
	return nil
}
