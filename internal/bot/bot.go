package bot

import (
	"log"

	"go-vpn-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// RunBot - запуск бота
func RunBot(database *database.DB, botToken string) {
	// Создаем объект бота
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	// Включаем режим дебага (опционально)
	bot.Debug = true

	log.Printf("Бот запущен: %s", bot.Self.UserName)

	// Создаем обработчик
	handler := &BotHandler{
		Bot: bot,
		DB:  database,
	}

	// Настраиваем получение обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Обработка обновлений
	for update := range updates {
		if update.Message != nil {
			handler.HandleMessage(update.Message)
		}
	}
}