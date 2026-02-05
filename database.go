package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✓ Conexión a PostgreSQL establecida")
	return db, nil
}

func Migrate(db *sql.DB) error {
	queries := []string{
		// Tabla de usuarios
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Tabla de chats (siempre entre 2 usuarios)
		`CREATE TABLE IF NOT EXISTS chats (
			id SERIAL PRIMARY KEY,
			user1_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			user2_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user1_id, user2_id),
			CHECK (user1_id < user2_id)
		)`,

		// Tabla de mensajes
		`CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			chat_id INTEGER REFERENCES chats(id) ON DELETE CASCADE,
			sender_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Índices para mejor rendimiento
		`CREATE INDEX IF NOT EXISTS idx_chats_users ON chats(user1_id, user2_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id)`,
	}

	for i, query := range queries {
		if _, err := db.Exec(query); err != nil {
			log.Printf("❌ Error en migración %d: %v", i+1, err)
			return err
		}
	}

	log.Println("✓ Tablas creadas/verificadas")
	return nil
}