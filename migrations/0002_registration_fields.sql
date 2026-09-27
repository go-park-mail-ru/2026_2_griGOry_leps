-- +goose Up
ALTER TABLE users ALTER COLUMN firstname SET NOT NULL;
ALTER TABLE users ALTER COLUMN nickname SET NOT NULL;
ALTER TABLE users ALTER COLUMN phonenumber SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_phonenumber_key UNIQUE (phonenumber);

-- +goose Down
ALTER TABLE users DROP CONSTRAINT users_phonenumber_key;
ALTER TABLE users ALTER COLUMN phonenumber DROP NOT NULL;
ALTER TABLE users ALTER COLUMN nickname DROP NOT NULL;
ALTER TABLE users ALTER COLUMN firstname DROP NOT NULL;
