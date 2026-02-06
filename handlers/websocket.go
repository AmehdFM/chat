package handlers

import (
	"chat/config"
	"chat/models"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Permitir todas las conexiones
		},
	}
	// Mapa de conexiones activas: userID -> websocket connection
	connections = make(map[int]*websocket.Conn)
	connMutex   sync.RWMutex
)

type WebSocketHandler struct {
	db *sql.DB
}

func NewWebSocketHandler(db *sql.DB) *WebSocketHandler {
	return &WebSocketHandler{db: db}
}

// HandleStream - Para uso con middleware (header Authorization)
func (h *WebSocketHandler) HandleStream(c *gin.Context) {
	userID := c.GetInt("user_id")
	username := c.GetString("username")

	h.handleWebSocketConnection(c, userID, username)
}

// HandleStreamWithToken - Para uso con token en query param (NUEVO)
func (h *WebSocketHandler) HandleStreamWithToken(c *gin.Context) {
	// Extraer token del query param
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token requerido en query param"})
		return
	}

	// Verificar token manualmente
	cfg := config.Load()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado"})
		return
	}

	// Extraer claims
	var userID int
	var username string
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		userID = int(claims["user_id"].(float64))
		username = claims["username"].(string)
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Claims inválidos"})
		return
	}

	h.handleWebSocketConnection(c, userID, username)
}

// handleWebSocketConnection - Lógica común de conexión WebSocket
func (h *WebSocketHandler) handleWebSocketConnection(c *gin.Context, userID int, username string) {
	// Actualizar conexión a WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ Error actualizando a WebSocket: %v", err)
		return
	}

	// Registrar conexión
	connMutex.Lock()
	connections[userID] = conn
	connMutex.Unlock()

	log.Printf("✅ Usuario %s (ID: %d) conectado al stream", username, userID)

	// Enviar mensaje de bienvenida
	welcomeMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Conectado al stream",
		"user_id": userID,
	}
	conn.WriteJSON(welcomeMsg)

	// Limpiar al desconectar
	defer func() {
		connMutex.Lock()
		delete(connections, userID)
		connMutex.Unlock()
		conn.Close()
		log.Printf("❌ Usuario %s (ID: %d) desconectado", username, userID)
	}()

	// Mantener conexión viva y escuchar mensajes del cliente
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("⚠️ Error leyendo mensaje de %s: %v", username, err)
			break
		}

		// Responder a ping con pong
		if messageType == websocket.PingMessage {
			conn.WriteMessage(websocket.PongMessage, nil)
		}
	}
}

// notifyNewMessage - Enviar mensaje nuevo por WebSocket al receptor
func notifyNewMessage(userID int, message models.Message) {
	connMutex.RLock()
	conn, exists := connections[userID]
	connMutex.RUnlock()

	if !exists {
		log.Printf("ℹ️ Usuario %d no está conectado, mensaje no enviado por WS", userID)
		return
	}

	// Crear payload con el tipo de evento
	payload := map[string]interface{}{
		"type":    "new_message",
		"message": message,
	}

	// Serializar a JSON
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Error serializando mensaje: %v", err)
		return
	}

	// Enviar por WebSocket
	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("❌ Error enviando mensaje a usuario %d: %v", userID, err)
		// Limpiar conexión muerta
		connMutex.Lock()
		delete(connections, userID)
		connMutex.Unlock()
		conn.Close()
	} else {
		log.Printf("✅ Mensaje enviado por WS a usuario %d", userID)
	}
}