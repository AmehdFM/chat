# Chat API | Mensajería en tiempo real con Go

Backend para integrar conversaciones individuales en una aplicación web o móvil. Combina una API REST para autenticación e historial con WebSocket para notificar mensajes nuevos, manteniendo los datos en PostgreSQL.

## Qué permite construir

Una base de mensajería para herramientas internas, atención o colaboración donde cada usuario pueda consultar sus conversaciones y recibir eventos en tiempo real. Este repositorio contiene el servidor y sus herramientas administrativas; el cliente visual se desarrolla por separado.

## Funcionalidades implementadas

- Inicio de sesión con contraseñas almacenadas mediante bcrypt y emisión de JWT.
- Conversaciones uno a uno, con orden canónico de participantes para evitar duplicados.
- Consulta de conversaciones e historial de mensajes.
- Comprobación de pertenencia al chat en las operaciones de mensajes.
- Notificaciones WebSocket al destinatario conectado.
- Creación de usuarios y conversaciones mediante comandos.
- Creación de tablas al iniciar el servidor y empaquetado Docker.

## Tecnologías y estructura

**Go · Gin · PostgreSQL · JWT · bcrypt · Gorilla WebSocket · Docker**

| Ruta | Responsabilidad |
|---|---|
| [main.go](main.go) | Arranque, rutas y comandos administrativos |
| [config/](config/) | Variables de entorno |
| [database/](database/) | Conexión y creación de tablas |
| [handlers/](handlers/) | Autenticación, conversaciones y WebSocket |
| [models/](models/) | Modelos de usuarios, chats y mensajes |
| [cmd/create-user/](cmd/create-user/) | Alta de usuarios con contraseña oculta |
| [Dockerfile](Dockerfile) | Compilación en etapas e imagen de ejecución |

## Ejecutar localmente

Requisitos: Go compatible con [go.mod](go.mod), que declara Go 1.21, y PostgreSQL. Crear previamente una base de desarrollo llamada `chat_db`.

Los comandos de entorno siguientes usan Bash:

```bash
git clone https://github.com/AmehdFM/chat.git
cd chat
go mod download
export DATABASE_URL="postgres://usuario:password@localhost:5432/chat_db?sslmode=disable"
export JWT_SECRET="$(openssl rand -hex 32)"
export PORT=8080
go run .
```

El servidor crea las tablas durante el arranque. Para preparar datos desde los comandos administrativos, iniciar primero el servidor una vez y después detenerlo o abrir otra terminal con las mismas variables.

| Variable | Uso |
|---|---|
| `DATABASE_URL` | Conexión PostgreSQL, obligatoria |
| `JWT_SECRET` | Firma de JWT; configurar un valor propio |
| `PORT` | Puerto HTTP; predeterminado: `8080` |

Los ejemplos de conexión son para una base local de desarrollo. Ajustar usuario, contraseña y TLS al entorno utilizado.

### Crear usuarios y una conversación

```bash
go run ./cmd/create-user
# Repetir para el segundo usuario.
go run . --create-chat=1,2
go run .
```

Sustituir `1,2` por los identificadores devueltos al crear los usuarios. El comando alternativo `go run . --create-user` también existe, pero muestra la contraseña al escribirla.

### Docker

La base debe ser accesible desde el contenedor. Establecer `DATABASE_URL` con ese host; `localhost` dentro del contenedor apunta al propio contenedor.

```bash
docker build -t chat-api .
docker run --rm -p 8080:8080 \
  -e DATABASE_URL -e JWT_SECRET -e PORT chat-api
```

## Recorrido de demostración

1. Crear dos usuarios y una conversación.
2. Iniciar sesión para obtener los tokens.
3. Abrir el WebSocket con uno de los usuarios.
4. Enviar un mensaje desde el otro y observar el evento.
5. Consultar el historial por REST para comprobar la persistencia.

## Estado y alcance

Implementación de backend para desarrollo y demostración. El repositorio no incluye una suite de pruebas automatizadas. Antes de un despliegue público, revisar orígenes WebSocket/CORS, validación de claims, gestión y duración de tokens, límites de solicitudes y concurrencia de conexiones. El código permite un secreto JWT predeterminado: configurar siempre `JWT_SECRET` y evitar registrar URLs con tokens.

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


## Autor

[Amehd Mendez](https://github.com/AmehdFM) · Backend e integración de sistemas.
