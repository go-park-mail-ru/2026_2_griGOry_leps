-- +goose Up
CREATE TABLE password_reset (
    token_hash char(64) PRIMARY KEY,
    user_id    integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX idx_password_reset_user_id ON password_reset (user_id, created_at);

-- +goose Down
DROP TABLE password_reset;
