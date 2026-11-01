-- +goose Up
CREATE TABLE orders (
    id                 integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    buyer_id           integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    idempotency_key    uuid NOT NULL,
    status             varchar(20) NOT NULL
                       CHECK (status IN ('awaiting_payment', 'placed', 'cancelled')),
    payment_method     varchar(10) NOT NULL CHECK (payment_method IN ('card', 'sbp', 'cod')),
    payment_status     varchar(20) NOT NULL
                       CHECK (payment_status IN ('pending', 'processing', 'paid', 'failed', 'on_delivery')),
    payment_token      varchar(64) UNIQUE,
    payment_confirm_at timestamptz,
    pay_until          timestamptz,
    recipient_name     varchar(50) NOT NULL,
    recipient_phone    varchar(20) NOT NULL,
    address_id         integer REFERENCES address(address_id) ON DELETE SET NULL,
    address_text       varchar(300),
    delivery_date      date,
    time_slot          varchar(20),
    comment            varchar(200) NOT NULL DEFAULT '',
    items_total        bigint NOT NULL CHECK (items_total >= 0),
    delivery_total     bigint NOT NULL CHECK (delivery_total >= 0),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (buyer_id, idempotency_key)
);

CREATE INDEX idx_orders_awaiting ON orders (pay_until) WHERE status = 'awaiting_payment';

CREATE TABLE order_seller (
    order_id  integer NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    seller_id integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    method    varchar(20) NOT NULL CHECK (method IN ('courier', 'pickup_point', 'self_pickup')),
    price     bigint NOT NULL CHECK (price >= 0),
    PRIMARY KEY (order_id, seller_id)
);

CREATE TABLE order_item (
    id        integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id  integer NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    ad_id     integer REFERENCES ad(id) ON DELETE SET NULL,
    seller_id integer NOT NULL REFERENCES user_account(user_id) ON DELETE CASCADE,
    title     varchar(200) NOT NULL,
    price     bigint NOT NULL CHECK (price >= 0),
    image_url varchar(255)
);

CREATE INDEX idx_order_item_order_id ON order_item (order_id);
CREATE INDEX idx_order_item_ad_id ON order_item (ad_id);

-- +goose Down
DROP TABLE order_item;
DROP TABLE order_seller;
DROP TABLE orders;
