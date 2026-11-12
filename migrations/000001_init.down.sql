DROP INDEX IF EXISTS idx_messages_content_trgm;

DROP INDEX IF EXISTS idx_chat_members_user_id;

DROP INDEX IF EXISTS idx_messages_chat_id_id_desc;

DROP TABLE IF EXISTS chat_members;

DROP TABLE IF EXISTS messages;

DROP TABLE IF EXISTS chats;

DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS pg_trgm;