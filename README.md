# Chat API

Backend REST + WebSocket para una aplicación de mensajería 1 a 1, escrito en Go con Gin, PostgreSQL y autenticación JWT.

## Características

- Autenticación mediante JWT (login con usuario y contraseña, hash con bcrypt)
- Chats 1 a 1 entre usuarios (sin duplicados, orden canónico de IDs)
- Historial de mensajes por chat
- Notificaciones en tiempo real vía WebSocket cuando llega un mensaje nuevo
- Migraciones automáticas de esquema al iniciar el servidor
- Herramientas de línea de comandos para crear usuarios y chats administrativamente

## Stack técnico

| Componente        | Tecnología                              |
|--------------------|------------------------------------------|
| Lenguaje           | Go 1.21                                   |
| Framework HTTP     | [Gin](https://github.com/gin-gonic/gin)   |
| Base de datos      | PostgreSQL (`lib/pq`)                     |
| WebSocket          | `gorilla/websocket`                       |
| Autenticación      | JWT (`golang-jwt/jwt/v5`) + bcrypt        |
| CORS               | `gin-contrib/cors`                        |

## Estructura del proyecto

```
chat/
├── main.go                    # Punto de entrada: servidor HTTP/WS y comandos CLI
├── config/
│   └── config.go              # Carga de configuración desde variables de entorno
├── database/
│   └── database.go            # Conexión a PostgreSQL y migraciones
├── handlers/
│   ├── auth.go                # Login, generación de JWT y middleware de autenticación
│   ├── chat.go                # Endpoints de chats y mensajes
│   └── websocket.go           # Manejo de conexiones WebSocket y notificaciones push
├── models/
│   └── models.go              # Structs: User, Chat, Message
├── cmd/
│   └── create-user/
│       └── main.go            # Binario independiente para crear usuarios con password oculto
├── Dockerfile
├── go.mod
└── go.sum
```

## Requisitos

- Go 1.21+
- PostgreSQL 13+

## Configuración

El servidor se configura mediante variables de entorno:

| Variable       | Requerida | Descripción                                   | Valor por defecto        |
|----------------|-----------|------------------------------------------------|---------------------------|
| `DATABASE_URL` | Sí        | Cadena de conexión a PostgreSQL                | —                          |
| `PORT`         | No        | Puerto donde escucha el servidor HTTP          | `8080`                     |
| `JWT_SECRET`   | No        | Secreto usado para firmar los tokens JWT       | `change-this-secret-key`   |

> **Importante:** cambia `JWT_SECRET` en producción. El valor por defecto solo existe para desarrollo local.

Ejemplo de `DATABASE_URL`:
```
postgres://usuario:password@localhost:5432/chat_db?sslmode=disable
```

## Instalación y ejecución

```bash
# Instalar dependencias
go mod download

# Definir variables de entorno
export DATABASE_URL="postgres://usuario:password@localhost:5432/chat_db?sslmode=disable"
export JWT_SECRET="un-secreto-fuerte"
export PORT=8080

# Ejecutar el servidor
go run main.go
```

Al iniciar en modo servidor, se crean automáticamente las tablas `users`, `chats` y `messages` si no existen.

## Docker

```bash
docker build -t chat-api .
docker run -e DATABASE_URL="postgres://usuario:password@host:5432/chat_db" -p 8080:8080 chat-api
```

## Herramientas de línea de comandos

El binario principal soporta dos modos administrativos además del modo servidor:

**Crear un usuario:**
```bash
go run main.go --create-user
```

**Crear un chat entre dos usuarios existentes:**
```bash
go run main.go --create-chat=1,2
```

También existe un binario independiente en `cmd/create-user`, equivalente al flag `--create-user` pero con el password oculto al escribirlo en terminal:
```bash
go run ./cmd/create-user
```

## API

Todas las rutas bajo `/api` excepto `/api/login` y `/api/stream` requieren el header:
```
Authorization: Bearer <token>
```

### `POST /api/login`
Autentica a un usuario y devuelve un token JWT.

**Body:**
```json
{
  "username": "juan",
  "password": "secreto123"
}
```

**Respuesta 200:**
```json
{
  "token": "eyJhbGciOi...",
  "user_id": 1,
  "username": "juan"
}
```

### `GET /api/chats`
Lista todos los chats del usuario autenticado, ordenados por el mensaje más reciente. Incluye el último mensaje y su fecha.

### `GET /api/chats/:chat_id/messages`
Devuelve el historial completo de mensajes de un chat, en orden cronológico. Verifica que el usuario pertenezca al chat.

### `POST /api/messages`
Envía un mensaje dentro de un chat existente.

**Body:**
```json
{
  "chat_id": 1,
  "content": "Hola!"
}
```

Al crearse el mensaje, se notifica en tiempo real al otro usuario si tiene una conexión WebSocket activa.

### `GET /api/stream?token=<jwt>`
Conexión WebSocket para recibir mensajes nuevos en tiempo real. El token JWT se pasa como query param (no como header) por limitaciones del protocolo WebSocket en el navegador.

Evento al conectar:
```json
{ "type": "connected", "message": "Conectado al stream", "user_id": 1 }
```

Evento al recibir un mensaje nuevo:
```json
{ "type": "new_message", "message": { "id": 5, "chat_id": 1, "sender_id": 2, "content": "Hola!", "created_at": "..." } }
```

## Modelo de datos

**users**
| Campo         | Tipo      |
|---------------|-----------|
| id            | SERIAL PK |
| username      | VARCHAR(50) UNIQUE |
| password_hash | VARCHAR(255) |
| created_at    | TIMESTAMP |

**chats**
| Campo      | Tipo      |
|------------|-----------|
| id         | SERIAL PK |
| user1_id   | INTEGER FK → users.id |
| user2_id   | INTEGER FK → users.id |
| created_at | TIMESTAMP |

`user1_id` siempre es menor que `user2_id` (se normaliza al crear el chat), lo que evita chats duplicados entre el mismo par de usuarios.

**messages**
| Campo      | Tipo      |
|------------|-----------|
| id         | SERIAL PK |
| chat_id    | INTEGER FK → chats.id |
| sender_id  | INTEGER FK → users.id |
| content    | TEXT |
| created_at | TIMESTAMP |

## Notas de seguridad

- Las contraseñas se almacenan con `bcrypt`, nunca en texto plano.
- CORS está actualmente configurado para aceptar cualquier origen (`AllowOrigins: []string{"*"}`); se recomienda restringirlo a los dominios del frontend antes de pasar a producción.
- El WebSocket acepta conexiones de cualquier origen (`CheckOrigin` siempre retorna `true`); considerar restringirlo también en producción.
