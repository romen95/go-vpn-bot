package bot

import (
	"fmt"

	"go-vpn-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotHandler struct {
	Bot *tgbotapi.BotAPI
	DB  *database.DB
}

func (h *BotHandler) HandleMessage(message *tgbotapi.Message) {
	switch message.Text {
	case "/start":
		h.handleStart(message)
	case "/balance":
		h.handleBalance(message)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неизвестная команда. Попробуйте /start или /balance.")
		h.Bot.Send(msg)
	}
}

func (h *BotHandler) handleStart(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		// Создаем нового пользователя
		err := h.DB.CreateUser(message.Chat.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании пользователя.")
			h.Bot.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, "Добро пожаловать! Чтобы воспользоваться тестовым периодом VPN, введите /get_config.")
		h.Bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы уже зарегистрированы. Чтобы воспользоваться тестовым периодом VPN, введите /get_config.")
		h.Bot.Send(msg)
	}
}

func (h *BotHandler) handleBalance(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		h.Bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш баланс: %.2f руб.", user.Balance))
	h.Bot.Send(msg)
}
