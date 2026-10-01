CREATE TABLE USER_ACCOUNT (
    user_id        SERIAL PRIMARY KEY,
    nickname       VARCHAR(64),
    first_name     VARCHAR(100) NOT NULL,
    last_name      VARCHAR(100) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    phone_number   VARCHAR(20),
    birthday       DATE,
    password_hash  VARCHAR(255) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    type           VARCHAR(20) NOT NULL
                   CHECK (type IN ('buyer', 'seller', 'both', 'admin'))
);

CREATE TABLE CATEGORY (
    category_id  SERIAL PRIMARY KEY,
    parent_id    INT REFERENCES CATEGORY(category_id) ON DELETE SET NULL,
    name         VARCHAR(100) NOT NULL,
    slug         VARCHAR(120) NOT NULL UNIQUE,
    image_url    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> category_id)
);

CREATE INDEX idx_category_parent_id ON CATEGORY(parent_id);

CREATE TABLE AD (
    id            SERIAL PRIMARY KEY,
    title         VARCHAR(200) NOT NULL,
    description   TEXT,
    price         NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    user_id       INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    status        VARCHAR(20) NOT NULL
                  CHECK (status IN ('draft', 'active', 'sold', 'archived', 'blocked')),
    category_id   INT REFERENCES CATEGORY(category_id) ON DELETE SET NULL,
    type          VARCHAR(20) NOT NULL
                  CHECK (type IN ('sell', 'buy', 'service')),
    city          VARCHAR(100),
    has_delivery  BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_ad_category_id ON AD(category_id);
CREATE INDEX idx_ad_city ON AD(city);
CREATE INDEX idx_ad_status_category_city ON AD(status, category_id, city);

CREATE TABLE AD_IMAGE (
    image_id    SERIAL PRIMARY KEY,
    ad_id       INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    position    INT NOT NULL DEFAULT 0 CHECK (position >= 0),
    alt_text    VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ad_image_ad_position ON AD_IMAGE(ad_id, position);

CREATE TABLE CART (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL UNIQUE REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE CART_ITEM (
    id       SERIAL PRIMARY KEY,
    cart_id  INT NOT NULL REFERENCES CART(id) ON DELETE CASCADE,
    ad_id    INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    UNIQUE (cart_id, ad_id)
);

CREATE TABLE FAVORITES (
    id       SERIAL PRIMARY KEY,
    user_id  INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    ad_id    INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    UNIQUE (user_id, ad_id)
);

CREATE TABLE AD_STATUS_EVENT (
    stat_id      SERIAL PRIMARY KEY,
    ad_id        INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    actor_id     INT REFERENCES USER_ACCOUNT(user_id) ON DELETE SET NULL,
    status       VARCHAR(20) NOT NULL
                 CHECK (status IN ('draft', 'active', 'sold', 'archived', 'blocked')),
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE PRICE_HISTORY (
    prh_id      SERIAL PRIMARY KEY,
    ad_id       INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    price       NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE CONVERSATION (
    room_id     SERIAL PRIMARY KEY,
    ad_id       INT REFERENCES AD(id) ON DELETE SET NULL,
    buyer_id    INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    seller_id   INT REFERENCES USER_ACCOUNT(user_id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE MESSAGE (
    message_id  SERIAL PRIMARY KEY,
    sender_id   INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    room_id     INT NOT NULL REFERENCES CONVERSATION(room_id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      VARCHAR(20) NOT NULL DEFAULT 'sent'
                CHECK (status IN ('sent', 'delivered', 'read'))
);

CREATE TABLE VIEWS (
    views_id  SERIAL PRIMARY KEY,
    user_id   INT REFERENCES USER_ACCOUNT(user_id) ON DELETE SET NULL,
    ad_id     INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    view_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE REVIEW (
    rew_id      SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    ad_id       INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    text        TEXT,
    rating      NUMERIC(3,2) NOT NULL CHECK (rating >= 0 AND rating <= 5),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, ad_id)
);

CREATE TABLE PROMOTION (
    prom_id          SERIAL PRIMARY KEY,
    ad_id            INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    when_paid        TIMESTAMPTZ NOT NULL DEFAULT now(),
    duration         DATE NOT NULL,
    promotion_level  INT NOT NULL CHECK (promotion_level >= 0)
);

CREATE TABLE "LIKE" (
    like_id  SERIAL PRIMARY KEY,
    user_id  INT NOT NULL REFERENCES USER_ACCOUNT(user_id) ON DELETE CASCADE,
    ad_id    INT NOT NULL REFERENCES AD(id) ON DELETE CASCADE,
    is_like  BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (user_id, ad_id)
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_user_account_updated_at
BEFORE UPDATE ON USER_ACCOUNT
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_ad_updated_at
BEFORE UPDATE ON AD
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_review_updated_at
BEFORE UPDATE ON REVIEW
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_category_updated_at
BEFORE UPDATE ON CATEGORY
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE FUNCTION log_price_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.price IS DISTINCT FROM OLD.price THEN
        INSERT INTO PRICE_HISTORY (ad_id, price, changed_at)
        VALUES (NEW.id, NEW.price, now());
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ad_price_history
AFTER UPDATE OF price ON AD
FOR EACH ROW EXECUTE FUNCTION log_price_change();

CREATE OR REPLACE FUNCTION set_conversation_seller()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.seller_id IS NULL AND NEW.ad_id IS NOT NULL THEN
        SELECT user_id INTO NEW.seller_id FROM AD WHERE id = NEW.ad_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_conversation_set_seller
BEFORE INSERT ON CONVERSATION
FOR EACH ROW EXECUTE FUNCTION set_conversation_seller();

CREATE OR REPLACE FUNCTION check_message_sender()
RETURNS TRIGGER AS $$
DECLARE
    v_buyer_id  INT;
    v_seller_id INT;
BEGIN
    SELECT buyer_id, seller_id
      INTO v_buyer_id, v_seller_id
      FROM CONVERSATION
     WHERE room_id = NEW.room_id;

    IF v_buyer_id IS NULL THEN
        RAISE EXCEPTION 'Комната % не найдена', NEW.room_id;
    END IF;

    IF NEW.sender_id <> v_buyer_id
       AND (v_seller_id IS NULL OR NEW.sender_id <> v_seller_id) THEN
        RAISE EXCEPTION
            'Пользователь % не является участником комнаты %',
            NEW.sender_id, NEW.room_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_message_sender_check
BEFORE INSERT OR UPDATE OF sender_id, room_id ON MESSAGE
FOR EACH ROW EXECUTE FUNCTION check_message_sender();
