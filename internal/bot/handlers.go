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
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
	}
}

func (h *BotHandler) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	log.Printf("Обработка callback: %s", callback.Data) // Лог для отладки
	switch callback.Data {
	case "get_started":
		user := h.DB.GetUserByID(callback.Message.Chat.ID)
		if user == nil {
			cfg, err := config.LoadConfig()
			if err != nil {
				log.Printf("Ошибка загрузки конфигурации: %v", err)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка загрузки конфигурации.")
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}
			// Создаем нового пользователя
			err = h.DB.CreateUser(callback.Message.Chat.ID, cfg.App.TestPeriodDays)
			if err != nil {
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при создании пользователя.")
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}
		}

		configUser := h.DB.GetUserConfig(callback.Message.Chat.ID)
		if configUser == "" {
			username := fmt.Sprintf("%d", callback.Message.Chat.ID)

			cfg, err := config.LoadConfig()
			if err != nil {
				log.Printf("Ошибка загрузки конфигурации: %v", err)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка загрузки конфигурации.")
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}

			// Отправляем запрос на создание пользователя в Marzban
			userResp, err := marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
			if err != nil {
				log.Printf("Получаем новый токен")
				// Получаем новый токен
				newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
				if err != nil {
					log.Printf("Не удалось получить новый токен: %v", err)
					msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
					h.Bot.Send(msg)
					return
				}

				err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey)
				if err != nil {
					log.Printf("Не удалось обновить конфиг: %v", err)
					msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при обновлении конфигурации.")
					h.Bot.Send(msg)
					return
				}

				// Повторяем запрос CreateUser
				cfg.Marzban.APIKey = newAPIKey // Обновляем APIKey в памяти
				userResp, err = marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
				if err != nil {
					log.Printf("Ошибка создания пользователя даже после обновления токена: %v", err)
					msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
					h.Bot.Send(msg)
					return
				}
			}

			if !userResp.Success {
				log.Printf("Не удалось создать пользователя в Marzban для %d: %s", callback.Message.Chat.ID, userResp.Message)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Ошибка создания конфигурации: %s", userResp.Message))
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}

			// Сохраняем конфиг в базе данных
			err = h.DB.UpdateUserConfig(callback.Message.Chat.ID, userResp.Message)
			if err != nil {
				log.Printf("Ошибка обновления конфига в базе для пользователя %d: %v", callback.Message.Chat.ID, err)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при сохранении конфигурации.")
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}
			configUser = h.DB.GetUserConfig(callback.Message.Chat.ID)
		}

		// Информация о сервисе
		guideText := "Гайд по установке\n\n" +
			"Выберите операционную систему\n\n" +
			"Вы увидите подробную инструкцию по настройке со ссылкой на скачивание приложения\n\n" +
			"Текущий сервер подключения:\n" +
			"🇳🇱 Нидерланды\n\n" +
			"🟢 Нажмите на КЛЮЧ и он автоматически скопируется:\n" +
			configUser

		// Создаем inline-кнопку
		buttonIOS := tgbotapi.NewInlineKeyboardButtonData("📱 iOS", "get_ios_guide")
		buttonAndroid := tgbotapi.NewInlineKeyboardButtonData("📱 Android", "get_android_guide")
		buttonWindows := tgbotapi.NewInlineKeyboardButtonData("🖥 Windows", "get_windows_guide")
		buttonMac := tgbotapi.NewInlineKeyboardButtonData("🖥 MacOS", "get_mac_guide")
		buttonChangeCountry := tgbotapi.NewInlineKeyboardButtonData("Сменить страну", "change_country")
		buttonBack := tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "go_back")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonIOS, buttonAndroid),
			tgbotapi.NewInlineKeyboardRow(buttonMac, buttonWindows),
			tgbotapi.NewInlineKeyboardRow(buttonChangeCountry),
			tgbotapi.NewInlineKeyboardRow(buttonBack),
		)

		// Отправляем сообщение с кнопкой
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, guideText)
		msg.ReplyMarkup = keyboard

		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки приветственного сообщения: %v", err)
		}

		// Отправляем ответ на callback
		callbackResp := tgbotapi.NewCallback(callback.ID, "Вы начали пользоваться сервисом!")
		if _, err := h.Bot.Request(callbackResp); err != nil {
			log.Printf("Ошибка отправки ответа на CallbackQuery: %v", err)
		}
	default:
		log.Printf("Неизвестное действие: %s", callback.Data)
	}
}

func (h *BotHandler) handleStart(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Printf("Ошибка загрузки конфигурации: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
			if _, err := h.Bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
			return
		}
		// Создаем нового пользователя
		err = h.DB.CreateUser(message.Chat.ID, cfg.App.TestPeriodDays)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании пользователя.")
			if _, err := h.Bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
			return
		}

	}

	// Информация о сервисе
	welcomeText := "Добро пожаловать в Boo VPN!\n\n" +
		"🔒 Безопасное соединение\n" +
		"🌍 Доступ к заблокированным сайтам\n" +
		"📈 Высокая скорость"

	// Создаем inline-кнопку
	button := tgbotapi.NewInlineKeyboardButtonData("Поехали!", "get_started")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(button),
	)

	// Отправляем сообщение с кнопкой
	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeText)
	msg.ReplyMarkup = keyboard

	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки приветственного сообщения: %v", err)
	}
}

func (h *BotHandler) HandleUpdate(update tgbotapi.Update) {
	// Логируем все обновления для отладки
	log.Printf("Обновление получено: %+v", update)

	if update.CallbackQuery != nil {
		log.Printf("Получен callback: %s", update.CallbackQuery.Data)
		h.handleCallbackQuery(update.CallbackQuery)
		return
	}

	if update.Message != nil {
		h.HandleMessage(update.Message)
	}
}

func (h *BotHandler) handleBalance(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш баланс: %.2f руб.", user.Balance))
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

func (h *BotHandler) handleGetConfig(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	if user.Config != "" {
		// Если конфиг уже существует, просто отправляем его
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("У вас уже есть конфиг:\n%s\n\nЧтобы получить новый конфиг, удалите старый, используя /delete_config.", user.Config))
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	// Генерируем имя пользователя на основе ID
	username := fmt.Sprintf("%d", message.Chat.ID)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Ошибка загрузки конфигурации: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	// Отправляем запрос на создание пользователя в Marzban
	userResp, err := marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		log.Printf("Получаем новый токен")
		// Получаем новый токен
		newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
		if err != nil {
			log.Printf("Не удалось получить новый токен: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
			h.Bot.Send(msg)
			return
		}

		err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey)
		if err != nil {
			log.Printf("Не удалось обновить конфиг: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при обновлении конфигурации.")
			h.Bot.Send(msg)
			return
		}

		// Повторяем запрос CreateUser
		cfg.Marzban.APIKey = newAPIKey // Обновляем APIKey в памяти
		userResp, err = marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
		if err != nil {
			log.Printf("Ошибка создания пользователя даже после обновления токена: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
			h.Bot.Send(msg)
			return
		}
	}

	if !userResp.Success {
		log.Printf("Не удалось создать пользователя в Marzban для %d: %s", message.Chat.ID, userResp.Message)
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка создания конфигурации: %s", userResp.Message))
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	// Сохраняем конфиг в базе данных
	err = h.DB.UpdateUserConfig(message.Chat.ID, userResp.Message)
	if err != nil {
		log.Printf("Ошибка обновления конфига в базе для пользователя %d: %v", message.Chat.ID, err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении конфигурации.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш конфиг успешно создан: %s", userResp.Message))
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

func (h *BotHandler) handleDeleteConfig(message *tgbotapi.Message) {
	user := h.DB.GetUserByID(message.Chat.ID)
	if user == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	if user.Config == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет сохранённого конфига.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	username := fmt.Sprintf("%d", message.Chat.ID)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Ошибка загрузки конфигурации: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка удаления пользователя: %v", err))
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	// Очищаем поле Config в базе данных
	err = h.DB.UpdateUserConfig(message.Chat.ID, "")
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка обновления базы данных.")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Ваш конфиг удалён. Вы можете создать новый, используя /get_config.")
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}
