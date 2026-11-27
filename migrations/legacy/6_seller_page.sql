-- +goose Up
ALTER TABLE review
    ADD COLUMN seller_id     integer REFERENCES user_account(user_id) ON DELETE CASCADE,
    ADD COLUMN listing_title varchar(200);

UPDATE review rv
SET seller_id = a.user_id, listing_title = a.title
FROM ad a
WHERE a.id = rv.ad_id;

UPDATE review SET rating = 1 WHERE rating < 1;

ALTER TABLE review
    ALTER COLUMN seller_id SET NOT NULL,
    ALTER COLUMN ad_id DROP NOT NULL,
    DROP CONSTRAINT review_ad_id_fkey,
    DROP CONSTRAINT review_rating_check;

ALTER TABLE review
    ADD CONSTRAINT review_ad_id_fkey FOREIGN KEY (ad_id) REFERENCES ad(id) ON DELETE SET NULL,
    ADD CONSTRAINT review_rating_check CHECK (rating >= 1 AND rating <= 5);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_review_seller()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.ad_id IS NOT NULL THEN
        SELECT user_id, title
          INTO NEW.seller_id, NEW.listing_title
          FROM ad
         WHERE id = NEW.ad_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_review_set_seller
BEFORE INSERT ON review
FOR EACH ROW EXECUTE FUNCTION set_review_seller();

CREATE INDEX idx_review_seller_id ON review (seller_id, created_at);
CREATE INDEX idx_review_ad_id ON review (ad_id);

CREATE TABLE review_image (
    image_id   integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rew_id     integer NOT NULL REFERENCES review(rew_id) ON DELETE CASCADE,
    url        text NOT NULL,
    position   integer NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rew_id, position)
);

CREATE TABLE seller_report (
    report_id   integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    seller_id   integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    reporter_id integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    reason      varchar(20) NOT NULL CHECK (reason IN ('fraud', 'prohibited', 'rude', 'other')),
    comment     varchar(500) NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (seller_id <> reporter_id)
);

CREATE INDEX idx_seller_report_seller_id ON seller_report (seller_id, created_at);

-- +goose Down
DROP TABLE seller_report, review_image;

DROP INDEX idx_review_ad_id;
DROP INDEX idx_review_seller_id;

DROP TRIGGER trg_review_set_seller ON review;
DROP FUNCTION set_review_seller();

DELETE FROM review WHERE ad_id IS NULL;

ALTER TABLE review
    DROP CONSTRAINT review_rating_check,
    DROP CONSTRAINT review_ad_id_fkey;

ALTER TABLE review
    ADD CONSTRAINT review_rating_check CHECK (rating >= 0 AND rating <= 5),
    ADD CONSTRAINT review_ad_id_fkey FOREIGN KEY (ad_id) REFERENCES ad(id) ON DELETE CASCADE,
    ALTER COLUMN ad_id SET NOT NULL,
    DROP COLUMN listing_title,
    DROP COLUMN seller_id;
