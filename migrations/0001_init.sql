-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    nickname TEXT,
    firstname TEXT,
    secondname TEXT,
    email TEXT NOT NULL UNIQUE,
    phonenumber TEXT,
    birthday DATE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rating NUMERIC(3,2) NOT NULL DEFAULT 0,
    review_count INT NOT NULL DEFAULT 0,
    type TEXT NOT NULL DEFAULT 'buyer'
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE sessions;
DROP TABLE users;
