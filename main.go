package main

import (
	"bufio"
	"chat/config"
	"chat/database"
	"chat/handlers"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 1. Definir banderas
	createUserMode := flag.Bool("create-user", false, "Activar modo creación de usuario")
	createChatMode := flag.String("create-chat", "", "ID de usuarios a conectar (ej: 1,2)")
	flag.Parse()

	cfg := config.Load()
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error conectando a PostgreSQL:", err)
	}
	defer db.Close()

	// 2. Lógica de comandos CLI
	if *createUserMode {
		runCreateUserCommand(db)
		return
	}

	if *createChatMode != "" {
		runCreateChatCommand(db, *createChatMode)
		return
	}

	// --- MODO SERVIDOR ---
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

	// Ruta pública de login
	router.POST("/api/login", authHandler.Login)

	// IMPORTANTE: WebSocket sin middleware (usa token en query param)
	router.GET("/api/stream", wsHandler.HandleStreamWithToken)

	// Rutas protegidas con middleware
	authorized := router.Group("/api")
	authorized.Use(authHandler.AuthMiddleware())
	{
		authorized.GET("/chats", chatHandler.GetUserChats)
		authorized.GET("/chats/:chat_id/messages", chatHandler.GetChatMessages)
		authorized.POST("/messages", chatHandler.SendMessage)
	}

	log.Printf("🚀 Servidor corriendo en puerto %s", cfg.Port)
	router.Run(":" + cfg.Port)
}

// Función para crear el usuario
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
	err := db.QueryRow("INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id", username, string(hash)).Scan(&userID)
	if err != nil {
		log.Fatal("❌ Error:", err)
	}

	fmt.Printf("\n✅ Usuario %s creado con ID: %d\n", username, userID)
}

// Crear chat entre dos IDs
func runCreateChatCommand(db *sql.DB, idsRaw string) {
	ids := strings.Split(idsRaw, ",")
	if len(ids) != 2 {
		log.Fatal("❌ Formato inválido. Usa: --create-chat=1,2")
	}

	u1, _ := strconv.Atoi(ids[0])
	u2, _ := strconv.Atoi(ids[1])

	// Aplicar lógica de ordenamiento
	if u1 == u2 {
		log.Fatal("❌ No puedes crear un chat contigo mismo")
	}
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	var chatID int
	err := db.QueryRow(
		"INSERT INTO chats (user1_id, user2_id) VALUES ($1, $2) ON CONFLICT DO NOTHING RETURNING id",
		u1, u2,
	).Scan(&chatID)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("⚠️ El chat ya existía.")
		} else {
			log.Fatal("❌ Error creando chat:", err)
		}
		return
	}

	fmt.Printf("✅ Chat creado exitosamente con ID: %d entre usuarios %d y %d\n", chatID, u1, u2)
}