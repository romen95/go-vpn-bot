package bot

import (
	"fmt"
	"log"

	"go-vpn-bot/internal/database"
	"go-vpn-bot/internal/marzban"

	config "go-vpn-bot/configs"

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
	case "/get_config":
		h.handleGetConfig(message)
	case "/delete_config":
		h.handleDeleteConfig(message)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неизвестная команда. Попробуйте /start, /balance, /get_config или /delete_config.")
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

func (h *BotHandler) handleGetConfig(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		h.Bot.Send(msg)
		return
	}

	if user.Config != "" {
		// Если конфиг уже существует, просто отправляем его
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("У вас уже есть конфиг:\n%s\n\nЧтобы получить новый конфиг, удалите старый, используя /delete_config.", user.Config))
		h.Bot.Send(msg)
		return
	}

	// Генерируем имя пользователя на основе ID
	username := fmt.Sprintf("%d", message.Chat.ID)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}
	// Отправляем запрос на создание пользователя в Marzban
	userResp, err := marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		log.Printf("Ошибка создания пользователя в Marzban для %d: %v", message.Chat.ID, err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
		h.Bot.Send(msg)
		return
	}

	if !userResp.Success {
		log.Printf("Не удалось создать пользователя в Marzban для %d: %s", message.Chat.ID, userResp.Message)
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка создания конфигурации: %s", userResp.Message))
		h.Bot.Send(msg)
		return
	}

	// Сохраняем конфиг в базе данных
	err = h.DB.UpdateUserConfig(message.Chat.ID, userResp.Message)
	if err != nil {
		log.Printf("Ошибка обновления конфига в базе для пользователя %d: %v", message.Chat.ID, err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении конфигурации.")
		h.Bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш конфиг успешно создан: %s", userResp.Message))
	h.Bot.Send(msg)
}

func (h *BotHandler) handleDeleteConfig(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		h.Bot.Send(msg)
		return
	}

	if user.Config == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет сохранённого конфига.")
		h.Bot.Send(msg)
		return
	}

	username := fmt.Sprintf("%d", message.Chat.ID)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка удаления пользователя: %v", err))
		h.Bot.Send(msg)
		return
	}

	// Очищаем поле Config в базе данных
	err = h.DB.UpdateUserConfig(message.Chat.ID, "")
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка обновления базы данных.")
		h.Bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Ваш конфиг удалён. Вы можете создать новый, используя /get_config.")
	h.Bot.Send(msg)
}
