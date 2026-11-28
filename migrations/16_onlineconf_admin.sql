-- +goose Up
-- FK не даёт выдать право несуществующему id: его получил бы следующий зарегистрированный.
CREATE TABLE onlineconf_admin (
    user_id    integer PRIMARY KEY REFERENCES user_account(user_id) ON DELETE CASCADE,
    granted_by integer REFERENCES user_account(user_id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE onlineconf_admin;
