package main

import (
	"log"

	"go-vpn-bot/internal/bot"
	"go-vpn-bot/internal/database"

	config "go-vpn-bot/configs"
)

func main() {
	// Подключение к базе данных
	db, err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer db.Close()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Запуск Telegram-бота
	bot.RunBot(db, cfg.Bot.Token)
}
