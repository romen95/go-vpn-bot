package main

import (
	"log"

	"github.com/romen95/go-vpn-bot/bot"
	"github.com/romen95/go-vpn-bot/database"
)

func main() {
	// Подключаем базу данных
	database.Connect()

	// Запускаем бота
	err := bot.Start()
	if err != nil {
		log.Fatal("Failed to start bot:", err)
	}
}
