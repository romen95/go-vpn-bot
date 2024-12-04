package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

type DB struct {
	Conn *sql.DB
}

type User struct {
	ID                  int64
	Balance             float64
	IsTrial             bool
	IsActive            bool
	IsFriend            bool
	SubscriptionEndDate sql.NullTime
	Config1             string
	Config2             string
	Config3             string
}

// ConnectDB подключается к базе данных и создает таблицу, если она не существует
func ConnectDB() (*DB, error) {
	conn, err := sql.Open("sqlite", "vpn-bot.db")
	if err != nil {
		return nil, err
	}

	if err = conn.Ping(); err != nil {
		return nil, err
	}

	// Создаем таблицу, если она не существует
	err = createUsersTable(conn)
	if err != nil {
		return nil, err
	}

	return &DB{Conn: conn}, nil
}

// createUsersTable создает таблицу пользователей, если она не существует
func createUsersTable(conn *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		balance REAL DEFAULT 0,
		is_trial BOOLEAN DEFAULT FALSE,
		is_active BOOLEAN DEFAULT FALSE,
		is_friend BOOLEAN DEFAULT FALSE,
		subscription_end_date DATETIME DEFAULT NULL,
		config1 TEXT DEFAULT '',
		config2 TEXT DEFAULT '',
		config3 TEXT DEFAULT ''
	);
	`
	_, err := conn.Exec(query)
	if err != nil {
		log.Printf("Ошибка при создании таблицы: %v", err)
		return err
	}
	log.Println("Таблица пользователей успешно создана/обновлена")
	return nil
}

func (db *DB) Close() {
	db.Conn.Close()
}

func (db *DB) GetUserByID(userID int64) *User {
	var user User
	query := "SELECT id, balance, is_trial, is_active, is_friend, subscription_end_date, config1, config2, config3 FROM users WHERE id = ?"
	err := db.Conn.QueryRow(query, userID).Scan(&user.ID, &user.Balance, &user.IsTrial, &user.IsActive, &user.IsFriend, &user.SubscriptionEndDate, &user.Config1, &user.Config2, &user.Config3)
	if err != nil {
		return nil
	}
	return &user
}

func (db *DB) CreateUser(userID int64, trialDays int) error {
	trialEnd := time.Now().AddDate(0, 0, trialDays) // добавляем дни пробного периода
	query := "INSERT INTO users (id, balance, is_trial, is_active, is_friend, subscription_end_date, config1, config2, config3) VALUES (?, 0, TRUE, TRUE, FALSE, ?, '', '', '')"
	_, err := db.Conn.Exec(query, userID, trialEnd)
	return err
}

func (db *DB) UpdateUserBalance(userID int64, amount float64) error {
	query := "UPDATE users SET balance = balance + ? WHERE id = ?"
	_, err := db.Conn.Exec(query, amount, userID)
	return err
}

func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.Conn.Query("SELECT id, balance, is_trial, is_active, is_friend, subscription_end_date, config1, config2, config3 FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Balance, &user.IsTrial, &user.IsActive, &user.IsFriend, &user.SubscriptionEndDate, &user.Config1, &user.Config2, &user.Config3)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (db *DB) UpdateUserConfig(userID int64, configIndex int, value string) error {
	var query string
	switch configIndex {
	case 1:
		query = "UPDATE users SET config1 = ? WHERE id = ?"
	case 2:
		query = "UPDATE users SET config2 = ? WHERE id = ?"
	case 3:
		query = "UPDATE users SET config3 = ? WHERE id = ?"
	default:
		return fmt.Errorf("некорректный индекс конфига: %d", configIndex)
	}
	_, err := db.Conn.Exec(query, value, userID)
	return err
}

func (db *DB) GetUserConfig(userID int64, configIndex int) string {
	var query string
	var config string
	switch configIndex {
	case 1:
		query = "SELECT config1 FROM users WHERE id = ?"
	case 2:
		query = "SELECT config2 FROM users WHERE id = ?"
	case 3:
		query = "SELECT config3 FROM users WHERE id = ?"
	default:
		log.Printf("Неправильный индекс")
		return ""
	}
	err := db.Conn.QueryRow(query, userID).Scan(&config)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если пользователь не найден, возвращаем пустую строку
			return ""
		}
		// Логируем ошибку, если произошла другая проблема
		log.Printf("Ошибка при получении конфига пользователя с ID %d: %v", userID, err)
		return ""
	}

	return config
}

func (db *DB) UpdateTrialStatus(userID int64, isTrial bool) error {
	query := "UPDATE users SET is_trial = ? WHERE id = ?"
	_, err := db.Conn.Exec(query, isTrial, userID)
	return err
}

func (db *DB) UpdateActiveStatus(userID int64, isActive bool) error {
	query := "UPDATE users SET is_active = ? WHERE id = ?"
	_, err := db.Conn.Exec(query, isActive, userID)
	return err
}

func (db *DB) UpdateSubscriptionEndDate(userID int64, endDate time.Time) error {
	query := "UPDATE users SET subscription_end_date = ? WHERE id = ?"
	_, err := db.Conn.Exec(query, endDate, userID)
	return err
}
