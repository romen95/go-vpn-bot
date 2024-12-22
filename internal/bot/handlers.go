package bot

import (
	"fmt"
	"log"
	"runtime"
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

func logWithLocation(format string, args ...interface{}) {
	pc, file, line, ok := runtime.Caller(1)

	if !ok {
		log.Printf("Ошибка получения информации о месте вызова")
		return
	}

	funcName := runtime.FuncForPC(pc).Name()
	location := log.Prefix() + file + ":" + funcName + ":" + string(line)
	log.Printf(location+" "+format, args...)
}

func (h *BotHandler) StartDailySubscriptionCheck() {
	location, err := time.LoadLocation("Europe/Moscow")

	if err != nil {
		logWithLocation("Ошибка загрузки локации %v", err)
		return
	}

	for {
		now := time.Now().In(location)
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, location)

		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		duration := nextRun.Sub(now)
		log.Printf("Следующая проверка подписок запланирована на %v", nextRun)

		time.Sleep(duration)

		h.CheckSubscriptionsAndNotify()
	}
}

func (h *BotHandler) notifyUser(user database.User, message string) {
	msg := tgbotapi.NewMessage(user.ID, message)
	_, err := h.Bot.Send(msg)
	if err != nil {
		logWithLocation("Ошибка отправки уведомления пользователю %d: %v", user.ID, err)
	}
}

func (h *BotHandler) CheckSubscriptionsAndNotify() {
	users, err := h.DB.GetAllUsers()

	if err != nil {
		logWithLocation("Ошибка получения пользователей %v", err)
		return
	}

	var checkedCount, deletedCount int
	now := time.Now()

	for _, user := range users {
		subscriptionEnd := user.SubscriptionEndDate.Time
		daysLeft := int(subscriptionEnd.Sub(now).Hours()/24) + 1
		log.Printf("Осталось дней: %d", daysLeft)

		// Уведомление за 3 дня
		if daysLeft == 3 {
			h.notifyUser(user, "Ваша подписка истекает через 3 дня. Пожалуйста, продлите её, чтобы продолжить пользоваться услугами.")
		}

		// Уведомление за 1 день
		if daysLeft == 1 {
			h.notifyUser(user, "Внимание! Завтра истекает срок действия вашей подписки. Не забудьте оплатить!")
		}

		if !user.IsFriend && user.IsActive && subscriptionEnd.Before(now) {
			configs := []string{user.Config1, user.Config2, user.Config3}

			for i, configUser := range configs {
				if configUser != "" {
					cfg, err := config.LoadConfig()
					if err != nil {
						logWithLocation("Ошибка загрузки конфигурации: %v", err)
						return
					}

					err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, fmt.Sprintf("%d_device%d", user.ID, i+1))
					if err != nil {
						logWithLocation("Получаем новый токен")
						// Получаем новый токен
						newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
						if err != nil {
							logWithLocation("Не удалось получить новый токен: %v", err)
							return
						}

						err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey)
						if err != nil {
							logWithLocation("Не удалось обновить конфиг: %v", err)
							return
						}

						// Повторяем запрос CreateUser
						cfg.Marzban.APIKey = newAPIKey // Обновляем APIKey в памяти
						err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, fmt.Sprintf("%d_device%d", user.ID, i+1))
						if err != nil {
							logWithLocation("Ошибка удаления пользователя даже после обновления токена: %v", err)
							return
						}
					}
					h.DB.UpdateUserConfig(user.ID, i+1, "")
				}
				err = h.DB.UpdateTrialStatus(user.ID, false)
				if err != nil {
					logWithLocation("Ошибка обновления тестового статуса у пользователя %d: %v", user.ID, err)
					return
				}

				err = h.DB.UpdateActiveStatus(user.ID, false)
				if err != nil {
					logWithLocation("Ошибка обновления активного статуса у пользователя %d: %v", user.ID, err)
					return
				}
			}
			deletedCount++
		}
		if !user.IsFriend {
			checkedCount++
		}

		// Отправляем информацию в Telegram-канал
		h.SendCheckResults(checkedCount, deletedCount)
	}
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
		button := tgbotapi.NewInlineKeyboardButtonData("🚀 Поехали!", "get_started")
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

	trialEnd := user.SubscriptionEndDate.Time
	daysRemaining := int(trialEnd.Sub(time.Now()).Hours() / 24)
	var text string
	if user.IsTrial {
		text = fmt.Sprintf("Количество оставшихся дней до окончания тестового периода: %d\n\n"+
			"Вы можете оплатить подписку, оплаченный период добавится к текущему количеству оставшихся дней.", daysRemaining+1)
	}

	if !user.IsTrial {
		text = fmt.Sprintf("Количество оставшихся дней до окончания подписки: %d\n\n", daysRemaining+1)
	}

	if !user.IsActive {
		text = "Оплатите подписку, чтобы продолжить пользоваться сервисом."
	}

	if user.IsFriend {
		text = "Ты пользуешься сервисом бесплатно!"
	}

	// Создаем inline-кнопки для различных платформ
	buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
	buttonConfigs := tgbotapi.NewInlineKeyboardButtonData("📶 Мои конфиги", "get_config")
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

func (h *BotHandler) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	log.Printf("Обработка callback: %s", callback.Data) // Лог для отладки
	switch callback.Data {
	case "get_started":
		h.SendSubscriptionInfo(callback)
		configUser := h.DB.GetUserConfig(callback.Message.Chat.ID, 1)
		if configUser == "" {
			username := fmt.Sprintf("%d_device1", callback.Message.Chat.ID)

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
			err = h.DB.UpdateUserConfig(callback.Message.Chat.ID, 1, userResp.Message)
			if err != nil {
				log.Printf("Ошибка обновления конфига в базе для пользователя %d: %v", callback.Message.Chat.ID, err)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Произошла ошибка при сохранении конфигурации.")
				if _, err := h.Bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				}
				return
			}
			configUser = h.DB.GetUserConfig(callback.Message.Chat.ID, 1)
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
	case "get_main":
		user := h.DB.GetUserByID(callback.Message.Chat.ID)
		if user == nil {
			log.Printf("Ошибка ошибка получения пользователя")
			return
		}

		trialEnd := user.SubscriptionEndDate.Time
		daysRemaining := int(trialEnd.Sub(time.Now()).Hours() / 24)
		var text string
		if user.IsTrial {
			text = fmt.Sprintf("Количество оставшихся дней до окончания тестового периода: %d\n\n"+
				"Вы можете оплатить подписку, оплаченный период добавится к текущему количеству оставшихся дней.", daysRemaining+1)
		}

		if !user.IsTrial {
			text = fmt.Sprintf("Количество оставшихся дней до окончания подписки: %d\n\n", daysRemaining+1)
		}

		if !user.IsActive {
			text = "Оплатите подписку, чтобы продолжить пользоваться сервисом."
		}

		if user.IsFriend {
			text = "Ты пользуешься сервисом бесплатно!"
		}

		// Создаем inline-кнопки для различных платформ
		buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
		buttonConfigs := tgbotapi.NewInlineKeyboardButtonData("📶 Мои конфиги", "get_config")
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
	case "get_config":
		user := h.DB.GetUserByID(callback.Message.Chat.ID)
		if user == nil {
			log.Printf("Ошибка получения пользователя")
			return
		}

		text := "📶 Мои конфиги\n\n" +
			"Выберите существующий конфиг, либо создайте новый\\."

		button1 := tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device1")
		button2 := tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device2")
		button3 := tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device3")
		buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(button1),
			tgbotapi.NewInlineKeyboardRow(button2),
			tgbotapi.NewInlineKeyboardRow(button3),
			tgbotapi.NewInlineKeyboardRow(buttonMain),
		)

		configs := []string{user.Config1, user.Config2, user.Config3}

		if configs[0] != "" && configs[1] == "" && configs[2] == "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 1", "get_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] != "" && configs[1] != "" && configs[2] == "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 1", "get_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 2", "get_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] != "" && configs[1] != "" && configs[2] != "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 1", "get_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 2", "get_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 3", "get_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] == "" && configs[1] != "" && configs[2] == "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 2", "get_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] == "" && configs[1] != "" && configs[2] != "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 2", "get_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 3", "get_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] == "" && configs[1] == "" && configs[2] != "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 3", "get_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if configs[0] != "" && configs[1] == "" && configs[2] != "" {
			button1 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 1", "get_device1")
			button2 = tgbotapi.NewInlineKeyboardButtonData("➕ Добавить конфиг", "new_device2")
			button3 = tgbotapi.NewInlineKeyboardButtonData("📱 Открыть устройство 3", "get_device3")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(button1),
				tgbotapi.NewInlineKeyboardRow(button2),
				tgbotapi.NewInlineKeyboardRow(button3),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		if !user.IsActive {
			text = "Оплатите подписку, чтобы продолжить пользоваться сервисом\\."
			buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
			keyboard = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(buttonPay),
				tgbotapi.NewInlineKeyboardRow(buttonMain),
			)
		}

		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			text,
			keyboard,
		)

		editMsg.ParseMode = "MarkdownV2"

		if _, err := h.Bot.Send(editMsg); err != nil {
			log.Printf("Ошибка отправки меню с конфигами: %v", err)
		}

		// Отправляем ответ на callback
		callbackResp := tgbotapi.NewCallback(callback.ID, "Ответ готов!")
		if _, err := h.Bot.Request(callbackResp); err != nil {
			log.Printf("Ошибка отправки ответа на CallbackQuery: %v", err)
		}
	case "get_device1":
		h.handleDeviceCallback(callback, 1)
	case "get_device2":
		h.handleDeviceCallback(callback, 2)
	case "get_device3":
		h.handleDeviceCallback(callback, 3)
	case "delete_device1":
		h.handleDeleteDevice(callback, 1)
	case "delete_device2":
		h.handleDeleteDevice(callback, 2)
	case "delete_device3":
		h.handleDeleteDevice(callback, 3)
	case "new_device1":
		h.handleNewDevice(callback, 1)
	case "new_device2":
		h.handleNewDevice(callback, 2)
	case "new_device3":
		h.handleNewDevice(callback, 3)
	default:
		log.Printf("Неизвестное действие: %s", callback.Data)
	}
}

func (h *BotHandler) sendDeviceConfig(callback *tgbotapi.CallbackQuery, deviceNumber int, userConfig string) {
	var text string
	var keyboard tgbotapi.InlineKeyboardMarkup

	if userConfig == "" {
		text = fmt.Sprintf("📱 Устройство %d\n\nУ вас нет конфига для этого устройства\\.", deviceNumber)
		buttonPay := tgbotapi.NewInlineKeyboardButtonData("💳 Оплатить", "pay_method")
		buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonPay),
			tgbotapi.NewInlineKeyboardRow(buttonMain),
		)
	} else {
		text = fmt.Sprintf(
			"📱 Устройство %d\n\nТекущий сервер подключения:\n🇳🇱 Нидерланды\n\n🟢 Нажмите на данный конфиг и он скопируется автоматически:\n```\n%s\n```",
			deviceNumber,
			userConfig,
		)

		buttonDelete := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("❌ Удалить конфиг %d", deviceNumber), fmt.Sprintf("delete_device%d", deviceNumber))
		buttonBack := tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "get_config")
		buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonDelete),
			tgbotapi.NewInlineKeyboardRow(buttonBack),
			tgbotapi.NewInlineKeyboardRow(buttonMain),
		)
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"

	if _, err := h.Bot.Send(editMsg); err != nil {
		log.Printf("Ошибка отправки сообщения для устройства %d: %v", deviceNumber, err)
	}

	// Ответ на callback
	callbackResp := tgbotapi.NewCallback(callback.ID, "Ответ готов!")
	if _, err := h.Bot.Request(callbackResp); err != nil {
		log.Printf("Ошибка отправки ответа на CallbackQuery: %v", err)
	}
}

func (h *BotHandler) handleDeviceCallback(callback *tgbotapi.CallbackQuery, deviceNumber int) {
	user := h.DB.GetUserByID(callback.Message.Chat.ID)
	if user == nil {
		log.Printf("Ошибка получения пользователя")
		return
	}

	var userConfig string
	switch deviceNumber {
	case 1:
		userConfig = user.Config1
	case 2:
		userConfig = user.Config2
	case 3:
		userConfig = user.Config3
	default:
		log.Printf("Некорректный номер устройства: %d", deviceNumber)
		return
	}

	h.sendDeviceConfig(callback, deviceNumber, userConfig)
}

func (h *BotHandler) handleDeleteDevice(callback *tgbotapi.CallbackQuery, deviceNumber int) {
	user := h.DB.GetUserByID(callback.Message.Chat.ID)
	if user == nil {
		log.Printf("Ошибка получения пользователя")
		return
	}

	var userConfig string
	switch deviceNumber {
	case 1:
		userConfig = user.Config1
	case 2:
		userConfig = user.Config2
	case 3:
		userConfig = user.Config3
	default:
		log.Printf("Неверный номер устройства: %d", deviceNumber)
		return
	}

	text := fmt.Sprintf("📱 Устройство %d\n\nКонфиг для этого устройства удален\\.", deviceNumber)
	buttonBack := tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "get_config")
	buttonMain := tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttonBack),
		tgbotapi.NewInlineKeyboardRow(buttonMain),
	)

	if userConfig != "" {
		if err := deleteUserFromMarzban(user.ID, deviceNumber); err != nil {
			log.Printf("Ошибка удаления пользователя из Marzban: %v", err)
			text = fmt.Sprintf("📱 Устройство %d\n\nНе удалось удалить конфиг, попробуйте позже\\.", deviceNumber)
		} else {
			h.DB.UpdateUserConfig(user.ID, deviceNumber, "")
		}
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"

	if _, err := h.Bot.Send(editMsg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}

	callbackResp := tgbotapi.NewCallback(callback.ID, "Ответ готов!")
	if _, err := h.Bot.Request(callbackResp); err != nil {
		log.Printf("Ошибка отправки CallbackQuery: %v", err)
	}
}

func deleteUserFromMarzban(userID int64, deviceNumber int) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	username := fmt.Sprintf("%d_device%d", userID, deviceNumber)
	err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
		if err != nil {
			return fmt.Errorf("не удалось обновить токен: %w", err)
		}
		if err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey); err != nil {
			return fmt.Errorf("ошибка обновления конфигурации: %w", err)
		}

		cfg.Marzban.APIKey = newAPIKey
		if err = marzban.DeleteUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username); err != nil {
			return fmt.Errorf("ошибка удаления после обновления токена: %w", err)
		}
	}
	return nil
}

func (h *BotHandler) handleNewDevice(callback *tgbotapi.CallbackQuery, deviceNumber int) {
	userID := callback.Message.Chat.ID

	configUser := h.DB.GetUserConfig(userID, deviceNumber)
	if configUser != "" {
		log.Printf("Конфиг уже существует")
		return
	}

	userResp, err := createUserMarzban(userID, deviceNumber)
	if err != nil {
		log.Printf("Ошибка создания пользователя %v", err)
		return
	}

	// Сохраняем конфигурацию в базе данных
	if err := h.DB.UpdateUserConfig(userID, deviceNumber, userResp.Message); err != nil {
		log.Printf("Ошибка создания пользователя %v", err)
		return
	}

	// Формируем текст сообщения
	text := fmt.Sprintf(
		"📱 Устройство %d\n\nТекущий сервер подключения:\n🇳🇱 Нидерланды\n\n🟢 Нажмите на данный конфиг и он скопируется автоматически:\n```\n%s\n```",
		deviceNumber, userResp.Message,
	)

	// Создаём клавиатуру
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Удалить конфиг", fmt.Sprintf("delete_device%d", deviceNumber))),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "get_config")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏡 В главное меню", "get_main")),
	)

	// Отправляем сообщение с конфигом
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"

	if _, err := h.Bot.Send(editMsg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}

	callbackResp := tgbotapi.NewCallback(callback.ID, "Ответ готов!")
	if _, err := h.Bot.Request(callbackResp); err != nil {
		log.Printf("Ошибка отправки CallbackQuery: %v", err)
	}
}

func createUserMarzban(userID int64, deviceNumber int) (*marzban.UserResponse, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Ошибка загрузки конфигурации: %w", err)
	}

	username := fmt.Sprintf("%d_device%d", userID, deviceNumber)
	userResp, err := marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
	if err != nil {
		newAPIKey, err := marzban.GetAPIKey(cfg.Marzban.APIURL, cfg.Marzban.Username, cfg.Marzban.Password)
		if err != nil {
			return nil, fmt.Errorf("не удалось обновить токен: %w", err)
		}
		if err = marzban.UpdateAPIKey("configs/config.yaml", newAPIKey); err != nil {
			return nil, fmt.Errorf("ошибка обновления конфигурации: %w", err)
		}

		cfg.Marzban.APIKey = newAPIKey
		userResp, err = marzban.CreateUser(cfg.Marzban.APIURL, cfg.Marzban.APIKey, username)
		if err != nil {
			return nil, fmt.Errorf("ошибка удаления после обновления токена: %w", err)
		}
	}
	return userResp, nil
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
