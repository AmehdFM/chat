package handlers

import (
	"chat/models"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	db *sql.DB
}

func NewChatHandler(db *sql.DB) *ChatHandler {
	return &ChatHandler{db: db}
}

// GetUserChats - Obtener todos los chats del usuario
func (h *ChatHandler) GetUserChats(c *gin.Context) {
	userID := c.GetInt("user_id")

	rows, err := h.db.Query(`
		SELECT 
			c.id,
			c.user1_id,
			c.user2_id,
			c.created_at,
			CASE 
				WHEN c.user1_id = $1 THEN c.user2_id 
				ELSE c.user1_id 
			END as other_user_id,
			CASE 
				WHEN c.user1_id = $1 THEN u2.username 
				ELSE u1.username 
			END as other_username,
			(
				SELECT content 
				FROM messages 
				WHERE chat_id = c.id 
				ORDER BY created_at DESC 
				LIMIT 1
			) as last_message,
			(
				SELECT created_at 
				FROM messages 
				WHERE chat_id = c.id 
				ORDER BY created_at DESC 
				LIMIT 1
			) as last_message_at
		FROM chats c
		JOIN users u1 ON c.user1_id = u1.id
		JOIN users u2 ON c.user2_id = u2.id
		WHERE c.user1_id = $1 OR c.user2_id = $1
		ORDER BY last_message_at DESC NULLS LAST
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error obteniendo chats"})
		return
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		var lastMessage sql.NullString
		var lastMessageAt sql.NullTime

		err := rows.Scan(
			&chat.ID,
			&chat.User1ID,
			&chat.User2ID,
			&chat.CreatedAt,
			&chat.OtherUserID,
			&chat.OtherUsername,
			&lastMessage,
			&lastMessageAt,
		)

		if err != nil {
			continue
		}

		if lastMessage.Valid {
			chat.LastMessage = lastMessage.String
		}
		if lastMessageAt.Valid {
			chat.LastMessageAt = &lastMessageAt.Time
		}

		chats = append(chats, chat)
	}

	if chats == nil {
		chats = []models.Chat{}
	}

	c.JSON(http.StatusOK, chats)
}

// GetChatMessages - Obtener todos los mensajes de un chat
func (h *ChatHandler) GetChatMessages(c *gin.Context) {
	userID := c.GetInt("user_id")
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de chat inválido"})
		return
	}

	// Verificar que el usuario pertenece a este chat
	var exists bool
	err = h.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM chats 
			WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
		)
	`, chatID, userID).Scan(&exists)

	if err != nil || !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso a este chat"})
		return
	}

	// Obtener mensajes
	rows, err := h.db.Query(`
		SELECT id, chat_id, sender_id, content, created_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at ASC
	`, chatID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error obteniendo mensajes"})
		return
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Content, &msg.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	if messages == nil {
		messages = []models.Message{}
	}

	c.JSON(http.StatusOK, messages)
}

// SendMessage - Enviar un mensaje
func (h *ChatHandler) SendMessage(c *gin.Context) {
	userID := c.GetInt("user_id")

	var req struct {
		ReceiverID int    `json:"receiver_id" binding:"required"`
		Content    string `json:"content" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}

	// Verificar que el receptor existe
	var receiverExists bool
	err := h.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", req.ReceiverID).Scan(&receiverExists)
	if err != nil || !receiverExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario receptor no existe"})
		return
	}

	// No puedes enviarte mensajes a ti mismo
	if userID == req.ReceiverID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes enviarte mensajes a ti mismo"})
		return
	}

	// Obtener o crear el chat (siempre user1_id < user2_id)
	user1ID := userID
	user2ID := req.ReceiverID
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var chatID int
	err = h.db.QueryRow(`
		INSERT INTO chats (user1_id, user2_id)
		VALUES ($1, $2)
		ON CONFLICT (user1_id, user2_id) DO UPDATE SET user1_id = chats.user1_id
		RETURNING id
	`, user1ID, user2ID).Scan(&chatID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creando chat"})
		return
	}

	// Insertar mensaje
	var message models.Message
	err = h.db.QueryRow(`
		INSERT INTO messages (chat_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, chat_id, sender_id, content, created_at
	`, chatID, userID, req.Content).Scan(
		&message.ID,
		&message.ChatID,
		&message.SenderID,
		&message.Content,
		&message.CreatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error enviando mensaje"})
		return
	}

	// Notificar por WebSocket al receptor
	notifyNewMessage(req.ReceiverID, message)

	c.JSON(http.StatusCreated, message)
}