-- +goose Up
CREATE TABLE onlineconf_admin (
    user_id    integer PRIMARY KEY,
    granted_by integer,
    granted_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE onlineconf_admin;
