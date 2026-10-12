-- 0001_create_users.up.sql
--
-- The `users` table. Every other table that references a user points at
-- users.id; treat that column as immutable.
--
-- Email uniqueness is enforced with a unique index on lower(email) rather than
-- CITEXT. Both give case-insensitive matching, but CITEXT means installing a
-- database-wide extension whose type is only resolvable through search_path --
-- which breaks as soon as anything sets a non-default search_path. An expression
-- index is portable, needs no extension, and behaves identically here.

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Trimmed by the application; the DB rejects anything absurdly long.
    name            TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),

    -- Stored lowercased and trimmed by the application. The index below is the
    -- authority on uniqueness, so two concurrent signups cannot both win.
    email           TEXT        NOT NULL,

    -- Encoded as an Argon2id PHC string. Never a raw or reversible value.
    -- Nullable so a future OAuth-only account can exist without one.
    password_hash   TEXT,

    -- Bumped whenever the password changes. Refresh tokens snapshot this, so
    -- any token minted before a credential change is refused.
    credentials_changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Brute-force state. Reset on every successful login.
    failed_login_count    INTEGER     NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    last_login_at         TIMESTAMPTZ,

    -- Set when the account is closed. Login refuses non-NULL values.
    disabled_at     TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness. Rejects "Ada@x.com" vs "ada@x.com" while staying
-- usable for the equality lookups the login path does.
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

-- Supports the "who is currently locked out" admin query.
CREATE INDEX users_locked_until_idx ON users (locked_until)
    WHERE locked_until IS NOT NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- Refresh token store.
--
-- Access tokens are stateless (a signed JWT) and are never stored. Refresh
-- tokens must be stored, because revoking a session has to be possible.
--
-- Only the SHA-256 HASH of a token is kept. A database leak therefore does not
-- yield usable tokens -- the attacker still needs the plaintext value.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    -- SHA-256 of the opaque token, hex encoded.
    token_hash  TEXT        NOT NULL UNIQUE,

    -- Snapshot of credentials_changed_at at issue time. On refresh we compare
    -- this against users.credentials_changed_at; a mismatch means the password
    -- changed after this token was minted, so the family is dead.
    credentials_changed_at TIMESTAMPTZ NOT NULL,

    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,

    -- Set on use. A refresh token is single-use: presenting one that is already
    -- rotated is treated as theft (reuse detection).
    rotated_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,

    -- Context for abuse investigation. NULL when not supplied.
    user_agent  TEXT,
    ip_address  TEXT
);

CREATE INDEX refresh_tokens_user_id_idx    ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

-- Reaper index: find rows to delete in bulk.
CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (expires_at)
    WHERE revoked_at IS NULL AND rotated_at IS NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- Login throttling.
--
-- Keyed by whatever the caller controls. We track both IP and account so that
-- neither a single-IP spray nor a distributed run against one account gets a
-- free pass. See internal/throttle.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE login_attempts (
    id           BIGSERIAL PRIMARY KEY,

    -- 'ip' or 'account'.
    scope        TEXT        NOT NULL CHECK (scope IN ('ip', 'account')),

    -- SHA-256 of the identifier (IP address, or email). Hashed so a database
    -- leak does not enumerate visitor addresses or accounts.
    key          TEXT        NOT NULL,

    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    succeeded    BOOLEAN     NOT NULL DEFAULT FALSE
);

-- The only query we run: "failures in the last N minutes for this key".
CREATE INDEX login_attempts_lookup_idx ON login_attempts (scope, key, attempted_at DESC);

-- ─────────────────────────────────────────────────────────────────────────────
-- Keep updated_at honest without relying on every writer to remember.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_touch_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();