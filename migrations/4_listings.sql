-- +goose Up
ALTER TABLE ad
    ALTER COLUMN price DROP NOT NULL,
    ADD COLUMN is_free       boolean NOT NULL DEFAULT false,
    ADD COLUMN is_negotiable boolean NOT NULL DEFAULT false,
    ADD COLUMN condition     varchar(20)
                             CHECK (condition IN ('new', 'used_good', 'used_repair')),
    ADD COLUMN address       varchar(255),
    ADD COLUMN close_reason  varchar(20)
                             CHECK (close_reason IN ('sold_on_goget', 'sold_elsewhere', 'changed_mind', 'other')),
    ADD COLUMN published_at  timestamptz;

UPDATE ad SET published_at = created_at WHERE status IN ('active', 'closed', 'sold');

ALTER TABLE ad_image
    ALTER COLUMN ad_id DROP NOT NULL,
    ADD COLUMN user_id integer REFERENCES user_account(user_id) ON DELETE CASCADE;

UPDATE ad_image i SET user_id = a.user_id FROM ad a WHERE a.id = i.ad_id;

ALTER TABLE ad_image ALTER COLUMN user_id SET NOT NULL;

CREATE INDEX idx_ad_image_ad_id ON ad_image (ad_id);
CREATE INDEX idx_ad_image_unattached ON ad_image (created_at) WHERE ad_id IS NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_price_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.price IS NULL THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'INSERT' OR NEW.price IS DISTINCT FROM OLD.price THEN
        INSERT INTO price_history (ad_id, price, changed_at)
        VALUES (NEW.id, NEW.price, now());
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_price_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO price_history (ad_id, price, changed_at)
        VALUES (NEW.id, NEW.price, now());
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE' AND NEW.price IS DISTINCT FROM OLD.price THEN
        INSERT INTO price_history (ad_id, price, changed_at)
        VALUES (NEW.id, NEW.price, now());
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP INDEX idx_ad_image_unattached;
DROP INDEX idx_ad_image_ad_id;
DELETE FROM ad_image WHERE ad_id IS NULL;
ALTER TABLE ad_image
    DROP COLUMN user_id,
    ALTER COLUMN ad_id SET NOT NULL;

DELETE FROM ad WHERE price IS NULL;
ALTER TABLE ad
    DROP COLUMN published_at,
    DROP COLUMN close_reason,
    DROP COLUMN address,
    DROP COLUMN condition,
    DROP COLUMN is_negotiable,
    DROP COLUMN is_free,
    ALTER COLUMN price SET NOT NULL;
