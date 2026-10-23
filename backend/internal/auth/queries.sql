-- name: UpsertUser :one
-- Создаёт учётку или обновляет пароль и роль по логину.
INSERT INTO users (login, password_hash, role)
VALUES ($1, $2, $3)
ON CONFLICT (login) DO UPDATE
SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role
RETURNING id, login, password_hash, role;

-- name: GetUserByLogin :one
SELECT id, login, password_hash, role FROM users WHERE login = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetSession :one
-- Живая сессия вместе с пользователем.
SELECT s.token_hash, s.csrf_token, s.expires_at, u.id AS user_id, u.login, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > $2;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= $1;
