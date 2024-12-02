package bot

import (
	"fmt"
	"log"
	"time"

	"go-vpn-bot/internal/database"
	"go-vpn-bot/internal/marzban"

	config "go-vpn-bot/configs"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotHandler struct {
	Bot *tgbotapi.BotAPI
	DB  *database.DB
}

func (h *BotHandler) StartDailySubscriptionCheck() {
	// Настроим ежедневную задачу на 19:00 по МСК (UTC+3)
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("Ошибка при загрузке локации: %v", err)
	}
	for {
		now := time.Now().In(location)
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, location)

		// Если текущее время уже прошло 19:00, ставим на завтрашний день
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		duration := nextRun.Sub(now)
		log.Printf("Следующая проверка подписок запланирована на %v", nextRun)

		// Ожидаем до следующей 19:00
		time.Sleep(duration)

		// Выполняем проверку подписок
		h.CheckSubscriptionsAndNotify()
	}
}

func (h *BotHandler) CheckSubscriptionsAndNotify() {
	// Получаем список всех пользователей
	users, _ := h.DB.GetAllUsers()

	var checkedCount, deletedCount int

	// Проверяем каждого пользователя
	for _, user := range users {
		if user.SubscriptionEndDate.Time.Before(time.Now()) { // Если подписка истекла
			// Удаляем конфиг из базы данных
			err := h.DB.UpdateUserConfig(user.ID, "")
			if err != nil {
				log.Printf("Ошибка удаления конфигурации для пользователя %d: %v", user.ID, err)
				return
			}

			// Удаляем пользователя с Marzban
			cfg, err := config.LoadConfig()
			if err != nil {
				log.Printf("Ошибка загрузки конфигурации: %v", err)
				return
			}

			err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, fmt.Sprintf("%d", user.ID))
			if err != nil {
				log.Printf("Ошибка удаления пользователя с Marzban для %d: %v", user.ID, err)
				return
			}

			deletedCount++
		}
		checkedCount++
	}

	// Отправляем информацию в Telegram-канал
	h.SendCheckResults(checkedCount, deletedCount)
}

func (h *BotHandler) SendCheckResults(checkedCount, deletedCount int) {
	// ID вашего канала
	channelID := "-1002480497483" // Замените на ваш канал

	// Создаем сообщение
	messageText := fmt.Sprintf("Проверка подписок завершена.\nПроверено пользователей: %d\nУдалено пользователей: %d", checkedCount, deletedCount)

	// Отправляем сообщение в канал
	msg := tgbotapi.NewMessageToChannel(channelID, messageText)
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения в канал: %v", err)
	}
}

func (h *BotHandler) SendSubscriptionInfo(callback *tgbotapi.CallbackQuery) {
	// ID вашего канала
	channelID := "-1002480497483" // Замените на ваш канал

	// Создаем сообщение
	messageText := fmt.Sprintf("Пользователь %d получил тестовый период!", callback.Message.Chat.ID)

	// Отправляем сообщение в канал
	msg := tgbotapi.NewMessageToChannel(channelID, messageText)
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения в канал: %v", err)
	}
}

func (h *BotHandler) HandleMessage(message *tgbotapi.Message) {
	switch message.Text {
	case "/start":
		h.handleStart(message)
	case "/check":
		h.CheckSubscriptionsAndNotify()
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неизвестная команда. Введите /start")
		if _, err := h.Bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
	}
}

func (h *BotHandler) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	log.Printf("Обработка callback: %s", callback.Data) // Лог для отладки
	switch callback.Data {
	case "get_started":
		h.SendSubscriptionInfo(callback)
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
			"🟢 Нажмите на данный конфиг и он скопируется автоматически:\n" +
			"```\n" + configUser + "\n```"

		// Создаем inline-кнопку
		buttonIOS := tgbotapi.NewInlineKeyboardButtonData("📱 iOS", "get_ios_guide")
		buttonAndroid := tgbotapi.NewInlineKeyboardButtonData("📱 Android", "get_android_guide")
		buttonWindows := tgbotapi.NewInlineKeyboardButtonData("🖥 Windows", "get_windows_guide")
		buttonMac := tgbotapi.NewInlineKeyboardButtonData("🖥 MacOS", "get_mac_guide")
		buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonIOS, buttonAndroid),
			tgbotapi.NewInlineKeyboardRow(buttonMac, buttonWindows),
			tgbotapi.NewInlineKeyboardRow(buttonMain),
		)

		// Отправляем сообщение с кнопкой
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			guideText,
			keyboard,
		)

		editMsg.ParseMode = "MarkdownV2"

		if _, err := h.Bot.Send(editMsg); err != nil {
			log.Printf("Ошибка отправки приветственного сообщения: %v", err)
		}

		// Отправляем ответ на callback
		callbackResp := tgbotapi.NewCallback(callback.ID, "Вы начали пользоваться сервисом!")
		if _, err := h.Bot.Request(callbackResp); err != nil {
			log.Printf("Ошибка отправки ответа на CallbackQuery: %v", err)
		}
	case "get_config":
		configUser := h.DB.GetUserConfig(callback.Message.Chat.ID)
		text := "📶 Мой конфиг\n\n" +
			"Текущий сервер подключения:\n" +
			"🇳🇱 Нидерланды\n\n" +
			"🟢 Нажмите на данный конфиг и он скопируется автоматически:\n" +
			"```\n" + configUser + "\n```" +
			"\n\nВы можете оплатить подписку, оплаченный период добавится к текущему количеству оставшихся дней"
		if configUser == "" {
			text = "На данный момент у вас нет действйющего конфига.\n\n" +
				"Вы можете оплатить подписку и начать пользоваться сервисом."
		}

		buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
		buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonPay),
			tgbotapi.NewInlineKeyboardRow(buttonMain),
		)
		// Отправляем сообщение с кнопкой
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			text,
			keyboard,
		)

		editMsg.ParseMode = "MarkdownV2"

		if _, err := h.Bot.Send(editMsg); err != nil {
			log.Printf("Ошибка отправки приветственного сообщения: %v", err)
		}

		// Отправляем ответ на callback
		callbackResp := tgbotapi.NewCallback(callback.ID, "Ответ готов!")
		if _, err := h.Bot.Request(callbackResp); err != nil {
			log.Printf("Ошибка отправки ответа на CallbackQuery: %v", err)
		}
	case "get_main":
		var text string
		user := h.DB.GetUserByID(callback.Message.Chat.ID)
		if user != nil {
			trialEnd := user.SubscriptionEndDate.Time
			daysRemaining := int(trialEnd.Sub(time.Now()).Hours() / 24)
			if daysRemaining < 0 {
				daysRemaining = 0 // Пробный период завершён
			}

			text = fmt.Sprintf("Вам доступно %d дней бесплатного пробного периода.", daysRemaining)
		} else {
			// Если по какой-то причине нет пользователя
			text = "Ваш пробный период завершён."
		}

		// Создаем inline-кнопки для различных платформ
		buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
		buttonConfigs := tgbotapi.NewInlineKeyboardButtonData("📶 Мой конфиг", "get_config")
		buttonSupport := tgbotapi.NewInlineKeyboardButtonData("🆘 Написать в поддержку", "get_support")
		buttonGuide := tgbotapi.NewInlineKeyboardButtonData("⚙️ Инструкция использования", "get_guide")

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonPay),
			tgbotapi.NewInlineKeyboardRow(buttonConfigs),
			tgbotapi.NewInlineKeyboardRow(buttonSupport),
			tgbotapi.NewInlineKeyboardRow(buttonGuide),
		)

		// Отправляем сообщение о пробном периоде с кнопками
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			text,
			keyboard,
		)

		if _, err := h.Bot.Send(editMsg); err != nil {
			log.Printf("Ошибка отправки приветственного сообщения: %v", err)
		}

		// Отправляем ответ на callback
		callbackResp := tgbotapi.NewCallback(callback.ID, "Выполнен переход в главное меню")
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
		// Если пользователь не найден, создаем нового с 7 днями пробного периода
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Printf("Ошибка загрузки конфигурации: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
			if _, err := h.Bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
			return
		}
		// Создаем нового пользователя с тестовым периодом
		err = h.DB.CreateUser(message.Chat.ID, cfg.App.TestPeriodDays)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании пользователя.")
			if _, err := h.Bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
			return
		}

		// Сообщение для нового пользователя
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
		return
	}

	// Вычисляем оставшиеся дни пробного периода
	var text string
	if user != nil {
		trialEnd := user.SubscriptionEndDate.Time
		daysRemaining := int(trialEnd.Sub(time.Now()).Hours() / 24)
		if daysRemaining < 0 {
			daysRemaining = 0 // Пробный период завершён
		}

		text = fmt.Sprintf("Вам доступно %d дней бесплатного пробного периода.", daysRemaining)
	} else {
		// Если по какой-то причине нет пользователя
		text = "Ваш пробный период завершён."
	}

	// Создаем inline-кнопки для различных платформ
	buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
	buttonConfigs := tgbotapi.NewInlineKeyboardButtonData("📶 Мой конфиг", "get_config")
	buttonSupport := tgbotapi.NewInlineKeyboardButtonData("🆘 Написать в поддержку", "get_support")
	buttonGuide := tgbotapi.NewInlineKeyboardButtonData("⚙️ Инструкция использования", "get_guide")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttonPay),
		tgbotapi.NewInlineKeyboardRow(buttonConfigs),
		tgbotapi.NewInlineKeyboardRow(buttonSupport),
		tgbotapi.NewInlineKeyboardRow(buttonGuide),
	)

	// Отправляем сообщение о пробном периоде с кнопками
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = keyboard
	if _, err := h.Bot.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения о пробном периоде: %v", err)
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

// func (h *BotHandler) handleBalance(message *tgbotapi.Message) {
// 	user := h.DB.GetUserByID(message.Chat.ID)
// 	if user == nil {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш баланс: %.2f руб.", user.Balance))
// 	if _, err := h.Bot.Send(msg); err != nil {
// 		log.Printf("Ошибка отправки сообщения: %v", err)
// 	}
// }

// func (h *BotHandler) handleGetConfig(message *tgbotapi.Message) {
// 	user := h.DB.GetUserByID(message.Chat.ID)
// 	if user == nil {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	if user.Config != "" {
// 		// Если конфиг уже существует, просто отправляем его
// 		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("У вас уже есть конфиг:\n%s\n\nЧтобы получить новый конфиг, удалите старый, используя /delete_config.", user.Config))
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	// Генерируем имя пользователя на основе ID
// 	username := fmt.Sprintf("%d", message.Chat.ID)

// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Printf("Ошибка загрузки конфигурации: %v", err)
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	// Отправляем запрос на создание пользователя в Marzban
// 	userResp, err := marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
// 	if err != nil {
// 		log.Printf("Получаем новый токен")
// 		// Получаем новый токен
// 		newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
// 		if err != nil {
// 			log.Printf("Не удалось получить новый токен: %v", err)
// 			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
// 			h.Bot.Send(msg)
// 			return
// 		}

// 		err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey)
// 		if err != nil {
// 			log.Printf("Не удалось обновить конфиг: %v", err)
// 			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при обновлении конфигурации.")
// 			h.Bot.Send(msg)
// 			return
// 		}

// 		// Повторяем запрос CreateUser
// 		cfg.Marzban.APIKey = newAPIKey // Обновляем APIKey в памяти
// 		userResp, err = marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
// 		if err != nil {
// 			log.Printf("Ошибка создания пользователя даже после обновления токена: %v", err)
// 			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании VPN-конфигурации.")
// 			h.Bot.Send(msg)
// 			return
// 		}
// 	}

// 	if !userResp.Success {
// 		log.Printf("Не удалось создать пользователя в Marzban для %d: %s", message.Chat.ID, userResp.Message)
// 		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка создания конфигурации: %s", userResp.Message))
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	// Сохраняем конфиг в базе данных
// 	err = h.DB.UpdateUserConfig(message.Chat.ID, userResp.Message)
// 	if err != nil {
// 		log.Printf("Ошибка обновления конфига в базе для пользователя %d: %v", message.Chat.ID, err)
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении конфигурации.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваш конфиг успешно создан: %s", userResp.Message))
// 	if _, err := h.Bot.Send(msg); err != nil {
// 		log.Printf("Ошибка отправки сообщения: %v", err)
// 	}
// }

// func (h *BotHandler) handleDeleteConfig(message *tgbotapi.Message) {
// 	user := h.DB.GetUserByID(message.Chat.ID)
// 	if user == nil {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы не зарегистрированы. Используйте /start для начала.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	if user.Config == "" {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет сохранённого конфига.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	username := fmt.Sprintf("%d", message.Chat.ID)

// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Printf("Ошибка загрузки конфигурации: %v", err)
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки конфигурации.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
// 	if err != nil {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка удаления пользователя: %v", err))
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	// Очищаем поле Config в базе данных
// 	err = h.DB.UpdateUserConfig(message.Chat.ID, "")
// 	if err != nil {
// 		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка обновления базы данных.")
// 		if _, err := h.Bot.Send(msg); err != nil {
// 			log.Printf("Ошибка отправки сообщения: %v", err)
// 		}
// 		return
// 	}

// 	msg := tgbotapi.NewMessage(message.Chat.ID, "Ваш конфиг удалён. Вы можете создать новый, используя /get_config.")
// 	if _, err := h.Bot.Send(msg); err != nil {
// 		log.Printf("Ошибка отправки сообщения: %v", err)
// 	}
// }
