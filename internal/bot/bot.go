package bot

import (
	"log"
	"time"

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
	bot.Debug = false

	log.Printf("Бот запущен: %s", bot.Self.UserName)

	// Создаем обработчик
	handler := &BotHandler{
		Bot: bot,
		DB:  database,
	}

	go handler.StartDailySubscriptionCheck()

	// Настраиваем получение обновлений
	// u := tgbotapi.NewUpdate(0)
	// u.Timeout = 60

	// updates := bot.GetUpdatesChan(u)

	// // Обрабатываем каждое обновление
	// for update := range updates {
	// 	handler.HandleUpdate(update) // Вызываем метод для обработки обновлений
	// }

	// Настраиваем получение обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Сохраняем время запуска бота
	botStartTime := time.Now()

	// Обрабатываем каждое обновление
	for update := range updates {
		// Проверяем, является ли обновление старым
		if update.CallbackQuery != nil {
			callbackTime := time.Unix(int64(update.CallbackQuery.Message.Date), 0)
			if callbackTime.Before(botStartTime) {
				hint := tgbotapi.NewCallback(update.CallbackQuery.ID, "Бот был перезагружен, используйте /start")
				_, _ = bot.Request(hint)
				continue
			}
			// Обрабатываем актуальные callback'и
			handler.HandleUpdate(update)
			continue
		}

		if update.Message != nil {
			messageTime := time.Unix(int64(update.Message.Date), 0)
			if messageTime.Before(botStartTime) {
				continue
			}
			// Обрабатываем актуальные сообщения
			handler.HandleUpdate(update)
		}
	}
}
