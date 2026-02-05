package handlers

import (
	"chat-backend/models"
	"database/sql"
	"encoding/json"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Permitir todas las conexiones (ajusta en producción si necesitas)
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

// HandleStream - Manejar conexión WebSocket para streaming
func (h *WebSocketHandler) HandleStream(c *gin.Context) {
	userID := c.GetInt("user_id")
	username := c.GetString("username")

	// Actualizar conexión a WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Error actualizando a WebSocket: %v", err)
		return
	}

	// Registrar conexión
	connMutex.Lock()
	connections[userID] = conn
	connMutex.Unlock()

	log.Printf("✓ Usuario %s (ID: %d) conectado al stream", username, userID)

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
		log.Printf("✗ Usuario %s (ID: %d) desconectado", username, userID)
	}()

	// Mantener conexión viva y escuchar mensajes del cliente (ping/pong)
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Si es ping, responder con pong
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
		// Usuario no está conectado, no hay problema
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
		log.Printf("Error serializando mensaje: %v", err)
		return
	}

	// Enviar por WebSocket
	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("Error enviando mensaje a usuario %d: %v", userID, err)
		// Limpiar conexión muerta
		connMutex.Lock()
		delete(connections, userID)
		connMutex.Unlock()
		conn.Close()
	}
}