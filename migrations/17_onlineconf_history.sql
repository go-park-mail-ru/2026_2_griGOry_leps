-- +goose Up
CREATE TABLE onlineconf_entry_history (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entry_id    integer NOT NULL REFERENCES onlineconf_entries(id),
    actor_id    integer NOT NULL REFERENCES user_account(user_id),
    operation   text NOT NULL CHECK (operation IN ('create', 'update')),
    changed_at  timestamptz NOT NULL DEFAULT clock_timestamp(),
    before_data jsonb,
    after_data  jsonb NOT NULL,

    CHECK (
        (operation = 'create' AND before_data IS NULL) OR
        (operation = 'update' AND before_data IS NOT NULL)
    )
);

CREATE INDEX onlineconf_entry_history_entry_id_id_idx
    ON onlineconf_entry_history (entry_id, id DESC);

-- +goose Down
DROP TABLE onlineconf_entry_history;
