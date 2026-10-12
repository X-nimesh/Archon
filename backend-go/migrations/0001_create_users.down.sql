-- 0001_create_users.down.sql
--
-- Reverse of 0001. Drops in dependency order.

DROP TRIGGER IF EXISTS users_touch_updated_at ON users;
DROP FUNCTION IF EXISTS touch_updated_at();

DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;