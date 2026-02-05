package main

import (
	"chat-backend/config"
	"chat-backend/database"
	"chat-backend/handlers"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	// Conectar a PostgreSQL
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error conectando a PostgreSQL:", err)
	}
	defer db.Close()

	// Crear tablas automáticamente
	if err := database.Migrate(db); err != nil {
		log.Fatal("Error creando tablas:", err)
	}

	log.Println("Base de datos lista ✓")

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	wsHandler := handlers.NewWebSocketHandler(db)

	// Configurar Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS para Flutter
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Rutas públicas
	router.POST("/api/login", authHandler.Login)

	// Rutas protegidas (requieren token)
	authorized := router.Group("/api")
	authorized.Use(authHandler.AuthMiddleware())
	{
		// Obtener todos los chats del usuario
		authorized.GET("/chats", chatHandler.GetUserChats)
		
		// Obtener mensajes de un chat específico
		authorized.GET("/chats/:chat_id/messages", chatHandler.GetChatMessages)
		
		// Enviar mensaje
		authorized.POST("/messages", chatHandler.SendMessage)
		
		// WebSocket para streaming en tiempo real
		authorized.GET("/stream", wsHandler.HandleStream)
	}

	log.Printf("🚀 Servidor corriendo en puerto %s", cfg.Port)
	router.Run(":" + cfg.Port)
}main.go