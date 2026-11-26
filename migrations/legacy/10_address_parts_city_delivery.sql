-- +goose Up
ALTER TABLE address
    ADD COLUMN city      varchar(100) NOT NULL DEFAULT '',
    ADD COLUMN apartment varchar(20)  NOT NULL DEFAULT '';

ALTER TABLE orders ALTER COLUMN address_text TYPE varchar(600);

CREATE UNIQUE INDEX uq_city_name_lower ON city (lower(name));

ALTER TABLE category ADD COLUMN delivery_allowed boolean NOT NULL DEFAULT true;

UPDATE category SET delivery_allowed = false
WHERE slug IN ('auto', 'realty', 'rent', 'services', 'jobs');

UPDATE ad SET has_delivery = false
WHERE has_delivery AND category_id IN (SELECT category_id FROM category WHERE NOT delivery_allowed);

-- +goose Down
ALTER TABLE category DROP COLUMN delivery_allowed;

DROP INDEX uq_city_name_lower;

ALTER TABLE orders ALTER COLUMN address_text TYPE varchar(500) USING left(address_text, 500);

ALTER TABLE address
    DROP COLUMN apartment,
    DROP COLUMN city;
