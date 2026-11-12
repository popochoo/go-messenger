-- Включаем расширение для быстрого поиска по подстроке
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. Пользователи
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Чаты (личные и групповые)
CREATE TABLE chats (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(10) NOT NULL CHECK (type IN ('direct', 'group')),
    title VARCHAR(100),
    -- NULL для direct, обязателен для group
    -- Для direct-чатов храним ключ вида "minUserID_maxUserID", чтобы на уровне БД запретить дубли личных чатов
    direct_key VARCHAR(50) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Участники чатов и права
CREATE TABLE chat_members (
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(10) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    -- Для статуса "прочитано"
    is_muted BOOLEAN NOT NULL DEFAULT FALSE,
    -- Для расширения Mute
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

-- Индекс для быстрого получения списка чатов конкретного пользователя
CREATE INDEX idx_chat_members_user_id ON chat_members(user_id);

-- 4. Сообщения
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE
    SET
        NULL,
        content TEXT NOT NULL,
        reply_to_id BIGINT REFERENCES messages(id) ON DELETE
    SET
        NULL,
        is_edited BOOLEAN NOT NULL DEFAULT FALSE,
        is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Главный индекс для быстрой загрузки истории чата "вверх"
CREATE INDEX idx_messages_chat_id_id_desc ON messages(chat_id, id DESC);

-- GIN-индекс для быстрого поиска по тексту сообщений внутри чата
CREATE INDEX idx_messages_content_trgm ON messages USING gin (content gin_trgm_ops);