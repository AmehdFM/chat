package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	// Obtener DATABASE_URL de la variable de entorno
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL no está configurada")
	}

	// Conectar a la base de datos
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error conectando a PostgreSQL:", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Error verificando conexión:", err)
	}

	fmt.Println("✓ Conectado a PostgreSQL")
	fmt.Println("\n=== CREAR NUEVO USUARIO ===\n")

	// Leer username
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	if username == "" {
		log.Fatal("❌ El username no puede estar vacío")
	}

	// Leer password (oculta)
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal("Error leyendo password:", err)
	}
	password := string(passwordBytes)
	fmt.Println() // Nueva línea después del password

	if password == "" {
		log.Fatal("❌ El password no puede estar vacío")
	}

	// Hash del password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Error hasheando password:", err)
	}

	// Insertar usuario
	var userID int
	err = db.QueryRow(
		"INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id",
		username, string(hash),
	).Scan(&userID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			log.Fatal("❌ Este username ya existe")
		}
		log.Fatal("Error creando usuario:", err)
	}

	fmt.Printf("\n✅ Usuario creado exitosamente!\n")
	fmt.Printf("   ID: %d\n", userID)
	fmt.Printf("   Username: %s\n", username)
}