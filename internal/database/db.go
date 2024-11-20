package database

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite" // SQLite driver
)

type DB struct {
	Conn *sql.DB
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
		config TEXT DEFAULT ''
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
	query := "SELECT id, balance, config FROM users WHERE id = ?"
	err := db.Conn.QueryRow(query, userID).Scan(&user.ID, &user.Balance, &user.Config)
	if err != nil {
		return nil
	}
	return &user
}

func (db *DB) CreateUser(userID int64) error {
	query := "INSERT INTO users (id, balance, config) VALUES (?, 0, '')"
	_, err := db.Conn.Exec(query, userID)
	return err
}

func (db *DB) UpdateUserBalance(userID int64, amount float64) error {
	query := "UPDATE users SET balance = balance + ? WHERE id = ?"
	_, err := db.Conn.Exec(query, amount, userID)
	return err
}

func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.Conn.Query("SELECT id, balance, config FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Balance, &user.Config)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

type User struct {
	ID      int64
	Balance float64
	Config  string
}
