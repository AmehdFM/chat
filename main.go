package main

import (
	"bufio"
	"chat/config"
	"chat/database"
	"chat/handlers"
	"database/sql" // <--- AGREGADO
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // Asegura el driver para el QueryRow
	"golang.org/x/crypto/bcrypt"
)

func main() {
	createUserMode := flag.Bool("create-user", false, "Activar modo creación de usuario")
	flag.Parse()

	cfg := config.Load()
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error conectando a PostgreSQL:", err)
	}
	defer db.Close()

	if *createUserMode {
		runCreateUserCommand(db)
		return 
	}

	if err := database.Migrate(db); err != nil {
		log.Fatal("Error creando tablas:", err)
	}

	authHandler := handlers.NewAuthHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	wsHandler := handlers.NewWebSocketHandler(db)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	router.POST("/api/login", authHandler.Login)
	authorized := router.Group("/api")
	authorized.Use(authHandler.AuthMiddleware())
	{
		authorized.GET("/chats", chatHandler.GetUserChats)
		authorized.GET("/chats/:chat_id/messages", chatHandler.GetChatMessages)
		authorized.POST("/messages", chatHandler.SendMessage)
		authorized.GET("/stream", wsHandler.HandleStream)
	}

	log.Printf("🚀 Servidor corriendo en puerto %s", cfg.Port)
	router.Run(":" + cfg.Port)
}

// CORREGIDO: Usamos *sql.DB directamente
func runCreateUserCommand(db *sql.DB) {
	fmt.Println("\n=== MODO ADMINISTRADOR: CREAR USUARIO ===")
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	var userID int
	err := db.QueryRow(
		"INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id",
		username, string(hash),
	).Scan(&userID)

	if err != nil {
		log.Fatal("❌ Error:", err)
	}
	fmt.Printf("\n✅ Usuario %s creado con ID: %d\n", username, userID)
}