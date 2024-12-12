package db

import (
	"database/sql"
	"log"
	"time"
)

type Translation struct {
	ID         int64     `db:"id"`
	UserID     int64     `db:"user_id"`
	Username   string    `db:"username"`
	Original   string    `db:"original"`
	Translated string    `db:"translated"`
	FromLang   string    `db:"from_lang"`
	ToLang     string    `db:"to_lang"`
	CreatedAt  time.Time `db:"created_at"`
}

type Stats struct {
	UserID           int64     `db:"user_id"`
	Username         string    `db:"username"`
	TranslationCount int       `db:"translation_count"`
	LastUsed         time.Time `db:"last_used"`
}

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite3", "trans.db")

	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS translations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT,
			original TEXT,
			translated TEXT,
			from_lang TEXT,
			to_lang TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
	`)

	if err != nil {
		log.Fatal(err)
	}

	return db
}

func InitStatsTable(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS usage_stats (
			user_id INTEGER PRIMARY KEY,
			username TEXT,
			translation_count INTEGER DEFAULT 0,
			last_used DATETIME DEFAULT CURRENT_TIMESTAMP
			)
	`)

	if err != nil {
		log.Fatal(err)
	}
}
