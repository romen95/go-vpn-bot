package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	tele "gopkg.in/telebot.v3"
	_ "modernc.org/sqlite"
)

const (
	apiBaseURL   = "http://81.31.244.167:8000/api"
	apiAuthToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImFjY2VzcyI6InN1ZG8iLCJleHAiOjE3MzE4NjQ1NTZ9.gz1QSHHWuCwnmizjgkYaIVSntFKwpuFbKPKgpjoHU1w" // Токен доступа, если требуется
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", "db_name.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	err = SetupDatabase(db)
	if err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}

	// Настройка бота
	pref := tele.Settings{
		Token:  "7656241581:AAHo0Dt2RWKw93uYQNV4riWBaDRgfGR8ayw",
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Передаём базу данных в обработчики через замыкания
	bot.Handle("/start", func(c tele.Context) error {
		return handleStart(c, db)
	})

	bot.Handle("/get_config", func(c tele.Context) error {
		return handleGetConfig(c, db)
	})

	bot.Handle("/topup", func(c tele.Context) error {
		return handleTopUp(c, db)
	})

	fmt.Println("Bot is running...")
	bot.Start()
}

func SetupDatabase(db *sql.DB) error {
	// Создаём таблицу пользователей
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id INTEGER UNIQUE NOT NULL,
		username TEXT NOT NULL,
		balance REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(usersTable)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Создаём таблицу подписок
	subscriptionsTable := `
	CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		start_date DATETIME NOT NULL,
		end_date DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`
	_, err = db.Exec(subscriptionsTable)
	if err != nil {
		return fmt.Errorf("failed to create subscriptions table: %v", err)
	}

	return nil
}

func handleStart(c tele.Context, db *sql.DB) error {
	username := c.Sender().Username
	telegramID := c.Sender().ID

	// Регистрируем пользователя, если его ещё нет
	_, err := db.Exec(`
	INSERT OR IGNORE INTO users (telegram_id, username) 
	VALUES (?, ?)`, telegramID, username)
	if err != nil {
		return c.Send("Failed to register user.")
	}

	return c.Send("Welcome! Use /get_config to get your VPN configuration.")
}

func handleGetConfig(c tele.Context, db *sql.DB) error {
	telegramID := c.Sender().ID

	// Проверяем, существует ли пользователь
	var balance float64
	err := db.QueryRow("SELECT balance FROM users WHERE telegram_id = ?", telegramID).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Send("You are not registered. Use /start first.")
		}
		return c.Send("Failed to fetch user data.")
	}

	if balance <= 0 {
		return c.Send("Insufficient balance. Please top up your account.")
	}

	// Получаем конфиг от Marzban API
	config := getUserConfig("admin", apiAuthToken)
	if err != nil {
		log.Printf("Failed to get config for user %d: %v", telegramID, err)
		return c.Send("Failed to retrieve configuration. Please try again later.")
	}

	// Отправляем конфиг пользователю
	return c.Send(fmt.Sprintf("Your VPN configuration:\n\n%s", config))
}

func handleTopUp(c tele.Context, db *sql.DB) error {
	telegramID := c.Sender().ID

	// Симулируем пополнение на фиксированную сумму (например, 10 единиц)
	const topUpAmount = 10.0
	_, err := db.Exec("UPDATE users SET balance = balance + ? WHERE telegram_id = ?", topUpAmount, telegramID)
	if err != nil {
		return c.Send("Failed to top up balance.")
	}

	return c.Send(fmt.Sprintf("Your balance has been topped up by %.2f units.", topUpAmount))
}

func getUserConfig(userID string, token string) error {
	url := fmt.Sprintf("%s/users/%s/config", apiBaseURL, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем токен для авторизации
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Println("User config:", string(body))
	return nil
}
