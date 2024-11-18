package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite" // Подключаем драйвер modernc.org/sqlite
)

func Connect() {
	// Подключаемся к базе данных
	db, err := sql.Open("sqlite", "db_name.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to the database.")

	// Создаем таблицу, если она не существует
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		telegram_id INTEGER UNIQUE NOT NULL,
		balance REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	fmt.Println("Table 'users' has been created or already exists.")
}
