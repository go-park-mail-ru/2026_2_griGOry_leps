-- +goose Up
ALTER TABLE user_account
    ADD COLUMN email_verified boolean NOT NULL DEFAULT false,
    ADD COLUMN pending_email  varchar(255);

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
DROP TABLE email_verification;

ALTER TABLE user_account
    DROP COLUMN pending_email,
    DROP COLUMN email_verified;
