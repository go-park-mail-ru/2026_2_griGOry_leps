-- +goose Up
ALTER TABLE ad DROP CONSTRAINT ad_status_check;
ALTER TABLE ad ADD CONSTRAINT ad_status_check
    CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold'));

ALTER TABLE ad_status_event DROP CONSTRAINT ad_status_event_status_check;
ALTER TABLE ad_status_event ADD CONSTRAINT ad_status_event_status_check
    CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold'));

ALTER TABLE ad
    ADD COLUMN is_free      boolean NOT NULL DEFAULT false,
    ADD COLUMN close_reason varchar(20)
        CHECK (close_reason IN ('sold_on_goget', 'sold_elsewhere', 'changed_mind', 'other'));

CREATE INDEX idx_ad_user_status ON ad (user_id, status);
CREATE INDEX idx_review_ad_id ON review (ad_id);

CREATE TABLE address (
    address_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    street     varchar(200) NOT NULL,
    house      varchar(50) NOT NULL,
    comment    varchar(200) NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_address_user_id ON address (user_id, created_at);
CREATE UNIQUE INDEX uq_address_default ON address (user_id) WHERE is_default;

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
DROP TABLE seller_report, review_image, address;

DROP INDEX idx_review_ad_id;
DROP INDEX idx_ad_user_status;

ALTER TABLE ad
    DROP COLUMN close_reason,
    DROP COLUMN is_free;

DELETE FROM ad_status_event WHERE status = 'rejected';
ALTER TABLE ad_status_event DROP CONSTRAINT ad_status_event_status_check;
ALTER TABLE ad_status_event ADD CONSTRAINT ad_status_event_status_check
    CHECK (status IN ('draft', 'moderation', 'active', 'closed', 'sold'));

UPDATE ad SET status = 'draft' WHERE status = 'rejected';
ALTER TABLE ad DROP CONSTRAINT ad_status_check;
ALTER TABLE ad ADD CONSTRAINT ad_status_check
    CHECK (status IN ('draft', 'moderation', 'active', 'closed', 'sold'));
