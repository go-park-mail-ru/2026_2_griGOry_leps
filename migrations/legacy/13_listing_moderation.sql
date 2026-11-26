-- +goose Up
ALTER TABLE user_account ADD COLUMN is_moderator boolean NOT NULL DEFAULT false;

-- Databases used for the demo may already have the revision and decision history.
ALTER TABLE ad
    ADD COLUMN IF NOT EXISTS moderation_revision bigint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS rejection_reason varchar(500),
    ADD COLUMN moderation_requested_at timestamptz;

DROP TRIGGER IF EXISTS trg_ad_moderation_revision ON ad;
DROP FUNCTION IF EXISTS update_moderation_revision();
UPDATE ad SET moderation_requested_at = updated_at WHERE status = 'moderation';

-- +goose StatementBegin
CREATE FUNCTION track_ad_moderation() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status = 'moderation' THEN
            NEW.moderation_requested_at := clock_timestamp();
        END IF;
    ELSE
        NEW.moderation_revision := OLD.moderation_revision + 1;
        IF NEW.status = 'moderation' AND OLD.status <> 'moderation' THEN
            NEW.moderation_requested_at := clock_timestamp();
        END IF;
    END IF;
    IF NEW.status <> 'rejected' THEN
        NEW.rejection_reason := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_ad_moderation BEFORE INSERT OR UPDATE ON ad
FOR EACH ROW EXECUTE FUNCTION track_ad_moderation();

CREATE TABLE IF NOT EXISTS moderation_decision (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    moderator_id integer NOT NULL REFERENCES user_account(user_id),
    revision bigint NOT NULL,
    status varchar(20) NOT NULL CHECK (status IN ('active', 'rejected')),
    reason varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'active' AND reason IS NULL)
           OR (status = 'rejected' AND reason IS NOT NULL AND length(trim(reason)) > 0))
);

CREATE UNIQUE INDEX moderation_decision_ad_revision ON moderation_decision (ad_id, revision);
CREATE INDEX ad_moderation_fifo ON ad (moderation_requested_at, id) WHERE status = 'moderation';

-- +goose Down
DROP INDEX ad_moderation_fifo;
DROP INDEX moderation_decision_ad_revision;
DROP TRIGGER trg_ad_moderation ON ad;
DROP FUNCTION track_ad_moderation();
ALTER TABLE ad DROP COLUMN moderation_requested_at;
ALTER TABLE user_account DROP COLUMN is_moderator;
-- Keep review decisions and reasons, including history from the demo, on rollback.
