-- +goose Up
ALTER TABLE onlineconf_entry_history
    DROP CONSTRAINT onlineconf_entry_history_entry_id_fkey,
    DROP CONSTRAINT onlineconf_entry_history_operation_check,
    DROP CONSTRAINT onlineconf_entry_history_check,
    ALTER COLUMN after_data DROP NOT NULL,
    ADD CONSTRAINT onlineconf_entry_history_operation_check
        CHECK (operation IN ('create', 'update', 'delete')),
    ADD CONSTRAINT onlineconf_entry_history_data_check CHECK (
        (operation = 'create' AND before_data IS NULL AND after_data IS NOT NULL) OR
        (operation = 'update' AND before_data IS NOT NULL AND after_data IS NOT NULL) OR
        (operation = 'delete' AND before_data IS NOT NULL AND after_data IS NULL)
    );

-- +goose Down
DELETE FROM onlineconf_entry_history WHERE operation = 'delete';
DELETE FROM onlineconf_entry_history
    WHERE entry_id NOT IN (SELECT id FROM onlineconf_entries);
ALTER TABLE onlineconf_entry_history
    DROP CONSTRAINT onlineconf_entry_history_data_check,
    DROP CONSTRAINT onlineconf_entry_history_operation_check,
    ALTER COLUMN after_data SET NOT NULL,
    ADD CONSTRAINT onlineconf_entry_history_operation_check
        CHECK (operation IN ('create', 'update')),
    ADD CONSTRAINT onlineconf_entry_history_check CHECK (
        (operation = 'create' AND before_data IS NULL) OR
        (operation = 'update' AND before_data IS NOT NULL)
    ),
    ADD CONSTRAINT onlineconf_entry_history_entry_id_fkey
        FOREIGN KEY (entry_id) REFERENCES onlineconf_entries(id);
