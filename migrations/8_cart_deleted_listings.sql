-- +goose Up
ALTER TABLE cart_item ADD COLUMN seller_id integer REFERENCES user_account(user_id) ON DELETE CASCADE;
UPDATE cart_item ci SET seller_id = a.user_id FROM ad a WHERE a.id = ci.ad_id;
ALTER TABLE cart_item ALTER COLUMN seller_id SET NOT NULL;
ALTER TABLE cart_item DROP CONSTRAINT cart_item_ad_id_fkey;

-- +goose Down
DELETE FROM cart_item ci WHERE NOT EXISTS (SELECT 1 FROM ad a WHERE a.id = ci.ad_id);
ALTER TABLE cart_item
    ADD CONSTRAINT cart_item_ad_id_fkey FOREIGN KEY (ad_id) REFERENCES ad(id) ON DELETE CASCADE;
ALTER TABLE cart_item DROP COLUMN seller_id;
