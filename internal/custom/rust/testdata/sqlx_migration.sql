-- sqlx migration: 20230101000000_create_users.sql
CREATE TABLE IF NOT EXISTS users (
    id    BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE posts (
    id      BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    title   TEXT NOT NULL
);

ALTER TABLE users ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
