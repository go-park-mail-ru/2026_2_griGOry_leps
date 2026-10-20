-- +goose Up
CREATE TABLE city (
    city_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name    varchar(100) NOT NULL UNIQUE
);

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
    ('Новосибирск');

ALTER TABLE user_account
    ADD COLUMN city_id        integer REFERENCES city(city_id) ON DELETE SET NULL,
    ADD COLUMN about          varchar(500) NOT NULL DEFAULT '',
    ADD COLUMN avatar         varchar(100),
    ADD COLUMN phone_verified boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE user_account
    DROP COLUMN phone_verified,
    DROP COLUMN avatar,
    DROP COLUMN about,
    DROP COLUMN city_id;

DROP TABLE city;
