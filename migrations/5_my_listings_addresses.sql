-- +goose Up
ALTER TABLE ad DROP CONSTRAINT ad_status_check;
ALTER TABLE ad ADD CONSTRAINT ad_status_check
    CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold'));

ALTER TABLE ad_status_event DROP CONSTRAINT ad_status_event_status_check;
ALTER TABLE ad_status_event ADD CONSTRAINT ad_status_event_status_check
    CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold'));

CREATE INDEX idx_ad_user_status ON ad (user_id, status);

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

-- +goose Down
DROP TABLE address;

DROP INDEX idx_ad_user_status;

DELETE FROM ad_status_event WHERE status = 'rejected';
ALTER TABLE ad_status_event DROP CONSTRAINT ad_status_event_status_check;
ALTER TABLE ad_status_event ADD CONSTRAINT ad_status_event_status_check
    CHECK (status IN ('draft', 'moderation', 'active', 'closed', 'sold'));

UPDATE ad SET status = 'draft' WHERE status = 'rejected';
ALTER TABLE ad DROP CONSTRAINT ad_status_check;
ALTER TABLE ad ADD CONSTRAINT ad_status_check
    CHECK (status IN ('draft', 'moderation', 'active', 'closed', 'sold'));
