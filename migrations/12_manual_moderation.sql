-- +goose Up
ALTER TABLE ad
    ADD COLUMN moderation_revision bigint NOT NULL DEFAULT 1,
    ADD COLUMN rejection_reason varchar(500);

-- +goose StatementBegin
CREATE FUNCTION update_moderation_revision() RETURNS trigger AS $$
BEGIN
    NEW.moderation_revision := OLD.moderation_revision + 1;
    IF NEW.status <> 'rejected' THEN
        NEW.rejection_reason := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_ad_moderation_revision BEFORE UPDATE ON ad
FOR EACH ROW EXECUTE FUNCTION update_moderation_revision();

CREATE TABLE moderation_decision (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    moderator_id integer NOT NULL REFERENCES user_account(user_id),
    revision bigint NOT NULL,
    status varchar(20) NOT NULL CHECK (status IN ('active', 'rejected')),
    reason varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'rejected' OR length(trim(reason)) > 0 AND reason IS NOT NULL)
);

CREATE INDEX idx_ad_moderation_queue ON ad(id) WHERE status = 'moderation';

-- +goose Down
DROP INDEX idx_ad_moderation_queue;
DROP TABLE moderation_decision;
DROP TRIGGER trg_ad_moderation_revision ON ad;
DROP FUNCTION update_moderation_revision();
ALTER TABLE ad DROP COLUMN rejection_reason, DROP COLUMN moderation_revision;
