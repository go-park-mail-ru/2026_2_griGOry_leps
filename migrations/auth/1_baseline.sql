-- +goose Up
-- Схема сервиса auth: итог legacy-миграций 1-14 для таблиц аккаунтов.
-- Исторические миграции лежат в migrations/legacy и нужны только для переноса данных.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE city (
    city_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name    varchar(100) NOT NULL UNIQUE
);

CREATE UNIQUE INDEX uq_city_name_lower ON city (lower(name));

INSERT INTO city (name) VALUES
    ('Москва'),
    ('Санкт-Петербург'),
    ('Балашиха'),
    ('Химки'),
    ('Мытищи'),
    ('Подольск'),
    ('Королёв'),
    ('Люберцы'),
    ('Красногорск'),
    ('Одинцово'),
    ('Казань'),
    ('Нижний Новгород'),
    ('Екатеринбург'),
    ('Новосибирск')
ON CONFLICT DO NOTHING;

CREATE TABLE user_account (
    user_id        integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_name     varchar(100) NOT NULL,
    email          varchar(255) NOT NULL UNIQUE,
    phone_number   varchar(20) NOT NULL UNIQUE,
    birthday       date,
    password_hash  varchar(255) NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    type           varchar(20) NOT NULL DEFAULT 'client'
                   CHECK (type IN ('client', 'seller', 'both', 'admin')),
    city_id        integer REFERENCES city(city_id) ON DELETE SET NULL,
    about          varchar(500) NOT NULL DEFAULT '',
    avatar         varchar(100),
    phone_verified boolean NOT NULL DEFAULT false,
    email_verified boolean NOT NULL DEFAULT false,
    pending_email  varchar(255),
    is_moderator   boolean NOT NULL DEFAULT false
);

CREATE TRIGGER trg_user_account_updated_at
BEFORE UPDATE ON user_account
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE session (
    id text PRIMARY KEY,
    user_id integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);

CREATE INDEX idx_session_user_id ON session (user_id);

CREATE TABLE password_reset (
    token_hash char(64) PRIMARY KEY,
    user_id    integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX idx_password_reset_user_id ON password_reset (user_id, created_at);

CREATE TABLE email_verification (
    token_hash char(64) PRIMARY KEY,
    user_id    integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    email      varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX idx_email_verification_user_id ON email_verification (user_id, created_at);

-- +goose Down
DROP TABLE email_verification, password_reset, session, user_account, city;
-- set_updated_at() не удаляется: от неё могут зависеть триггеры ads, если обе схемы лежат в одной базе.
