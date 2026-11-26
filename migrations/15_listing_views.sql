-- +goose Up
ALTER TABLE views ADD COLUMN is_first boolean NOT NULL DEFAULT false;

UPDATE views SET is_first = true
WHERE views_id IN (
    SELECT DISTINCT ON (v.ad_id, v.user_id) v.views_id
    FROM views v JOIN ad a ON a.id = v.ad_id
    WHERE v.user_id <> a.user_id
    ORDER BY v.ad_id, v.user_id, v.view_at, v.views_id
);

CREATE UNIQUE INDEX idx_first_listing_view ON views (ad_id, user_id) WHERE is_first;

-- +goose Down
DROP INDEX idx_first_listing_view;
ALTER TABLE views DROP COLUMN is_first;

