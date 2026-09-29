-- +goose Up

ALTER TABLE users ADD CONSTRAINT users_nickname_key UNIQUE (nickname);
ALTER TABLE users ADD CONSTRAINT users_type_check
    CHECK (type IN ('buyer', 'seller', 'both', 'admin'));
ALTER TABLE users ADD CONSTRAINT users_rating_check
    CHECK (rating >= 0 AND rating <= 5);

CREATE INDEX sessions_user_id_idx    ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- ============================================================ категории

CREATE TABLE category (
    id          SERIAL PRIMARY KEY,
    parent_id   INT REFERENCES category (id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(100) NOT NULL UNIQUE,
    image_url   TEXT,
    position    INT NOT NULL DEFAULT 0          -- порядок в карусели и каталоге
);
CREATE INDEX category_parent_id_idx ON category (parent_id);

-- ============================================================ объявления

CREATE TABLE ad (
    id            SERIAL PRIMARY KEY,
    user_id       INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category_id   INT REFERENCES category (id) ON DELETE SET NULL,
    title         VARCHAR(200) NOT NULL,
    description   TEXT,
    price         NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    city          VARCHAR(100),
    has_delivery  BOOLEAN NOT NULL DEFAULT false,
    status        VARCHAR(20) NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'active', 'sold', 'archived', 'blocked')),
    type_ad       VARCHAR(20) NOT NULL,         -- допустимые значения ещё не определены
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ad_status_created_idx ON ad (status, created_at DESC);
CREATE INDEX ad_user_id_idx        ON ad (user_id);
CREATE INDEX ad_category_id_idx    ON ad (category_id);
CREATE INDEX ad_city_idx           ON ad (city);

-- Фото: файл лежит в S3, в БД только ключ объекта (например ads/1024/3f9c2e1a.jpg)
CREATE TABLE ad_image (
    id           SERIAL PRIMARY KEY,
    ad_id        INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    storage_key  TEXT NOT NULL,
    position     INT NOT NULL DEFAULT 0,        -- 0 = главное фото карточки
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ad_id, position)
);

CREATE TABLE ad_status_event (
    id          SERIAL PRIMARY KEY,
    ad_id       INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    actor_id    INT REFERENCES users (id) ON DELETE SET NULL,
    status      VARCHAR(20) NOT NULL
                CHECK (status IN ('draft', 'active', 'sold', 'archived', 'blocked')),
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ad_status_event_ad_id_idx ON ad_status_event (ad_id, changed_at);

CREATE TABLE price_history (
    id          SERIAL PRIMARY KEY,
    ad_id       INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    price       NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX price_history_ad_id_idx ON price_history (ad_id, changed_at);

CREATE TABLE promotion (
    id               SERIAL PRIMARY KEY,
    ad_id            INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    when_paid        TIMESTAMPTZ NOT NULL DEFAULT now(),
    end_date         TIMESTAMPTZ NOT NULL,
    promotion_level  INT NOT NULL CHECK (promotion_level >= 0),
    CHECK (end_date > when_paid)
);
CREATE INDEX promotion_ad_id_idx ON promotion (ad_id, end_date);

-- ============================================================ активность покупателей

CREATE TABLE favorites (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ad_id       INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, ad_id)
);
CREATE INDEX favorites_ad_id_idx ON favorites (ad_id);

CREATE TABLE views (
    id       SERIAL PRIMARY KEY,
    user_id  INT REFERENCES users (id) ON DELETE SET NULL,   -- NULL для гостя
    ad_id    INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    view_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX views_ad_id_idx   ON views (ad_id, view_at);
CREATE INDEX views_user_id_idx ON views (user_id, view_at);

CREATE TABLE ad_like (
    id       SERIAL PRIMARY KEY,
    user_id  INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ad_id    INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    is_like  BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (user_id, ad_id)
);

CREATE TABLE review (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ad_id       INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    text        TEXT,
    rating      NUMERIC(3,2) NOT NULL CHECK (rating >= 0 AND rating <= 5),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, ad_id)
);
CREATE INDEX review_ad_id_idx ON review (ad_id);

-- ============================================================ корзина

CREATE TABLE cart (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cart_item (
    id       SERIAL PRIMARY KEY,
    cart_id  INT NOT NULL REFERENCES cart (id) ON DELETE CASCADE,
    ad_id    INT NOT NULL REFERENCES ad (id) ON DELETE CASCADE,
    UNIQUE (cart_id, ad_id)
);

-- ============================================================ чаты

CREATE TABLE conversation (
    id          SERIAL PRIMARY KEY,
    ad_id       INT REFERENCES ad (id) ON DELETE SET NULL,
    buyer_id    INT REFERENCES users (id) ON DELETE SET NULL,
    seller_id   INT REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ad_id, buyer_id)                    -- один чат покупателя по объявлению
);
CREATE INDEX conversation_buyer_id_idx  ON conversation (buyer_id);
CREATE INDEX conversation_seller_id_idx ON conversation (seller_id);

CREATE TABLE message (
    id          SERIAL PRIMARY KEY,
    room_id     INT NOT NULL REFERENCES conversation (id) ON DELETE CASCADE,
    sender_id   INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'sent'
                CHECK (status IN ('sent', 'delivered', 'read')),
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX message_room_id_idx ON message (room_id, sent_at);

-- ============================================================ сброс пароля

CREATE TABLE password_reset (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash   TEXT NOT NULL,                  -- хэш 6-значного кода, сам код не хранится
    attempts    INT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_user_id_idx ON password_reset (user_id, created_at DESC);

-- ============================================================ триггеры

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER users_set_updated_at  BEFORE UPDATE ON users  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER ad_set_updated_at     BEFORE UPDATE ON ad     FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER review_set_updated_at BEFORE UPDATE ON review FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- История цены: запись при создании объявления и при каждом изменении цены
-- +goose StatementBegin
CREATE FUNCTION log_price_change() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.price IS DISTINCT FROM OLD.price THEN
        INSERT INTO price_history (ad_id, price) VALUES (NEW.id, NEW.price);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER ad_log_price_change
AFTER INSERT OR UPDATE OF price ON ad
FOR EACH ROW EXECUTE FUNCTION log_price_change();

-- История статуса: запись при создании объявления и при каждой смене статуса
-- +goose StatementBegin
CREATE FUNCTION log_ad_status_change() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO ad_status_event (ad_id, actor_id, status) VALUES (NEW.id, NEW.user_id, NEW.status);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER ad_log_status_change
AFTER INSERT OR UPDATE OF status ON ad
FOR EACH ROW EXECUTE FUNCTION log_ad_status_change();

-- Продавец в чате подставляется из объявления, если не передан явно
-- +goose StatementBegin
CREATE FUNCTION set_conversation_seller() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.seller_id IS NULL AND NEW.ad_id IS NOT NULL THEN
        SELECT user_id INTO NEW.seller_id FROM ad WHERE id = NEW.ad_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER conversation_set_seller
BEFORE INSERT ON conversation
FOR EACH ROW EXECUTE FUNCTION set_conversation_seller();

-- Писать в чат могут только его участники
-- +goose StatementBegin
CREATE FUNCTION check_message_sender() RETURNS TRIGGER AS $$
DECLARE
    v_buyer_id  INT;
    v_seller_id INT;
BEGIN
    SELECT buyer_id, seller_id INTO v_buyer_id, v_seller_id
      FROM conversation WHERE id = NEW.room_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Комната % не найдена', NEW.room_id;
    END IF;

    IF NEW.sender_id IS DISTINCT FROM v_buyer_id
       AND NEW.sender_id IS DISTINCT FROM v_seller_id THEN
        RAISE EXCEPTION 'Пользователь % не является участником комнаты %', NEW.sender_id, NEW.room_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER message_check_sender
BEFORE INSERT OR UPDATE OF sender_id, room_id ON message
FOR EACH ROW EXECUTE FUNCTION check_message_sender();

-- Корзина создаётся автоматически при регистрации
-- +goose StatementBegin
CREATE FUNCTION create_cart_for_new_user() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO cart (user_id) VALUES (NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER users_create_cart
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION create_cart_for_new_user();

-- Корзины для пользователей, зарегистрированных до этой миграции
INSERT INTO cart (user_id) SELECT id FROM users ON CONFLICT (user_id) DO NOTHING;

-- +goose Down

DROP TRIGGER IF EXISTS users_create_cart     ON users;
DROP TRIGGER IF EXISTS users_set_updated_at  ON users;

DROP TABLE IF EXISTS password_reset;
DROP TABLE IF EXISTS message;
DROP TABLE IF EXISTS conversation;
DROP TABLE IF EXISTS cart_item;
DROP TABLE IF EXISTS cart;
DROP TABLE IF EXISTS review;
DROP TABLE IF EXISTS ad_like;
DROP TABLE IF EXISTS views;
DROP TABLE IF EXISTS favorites;
DROP TABLE IF EXISTS promotion;
DROP TABLE IF EXISTS price_history;
DROP TABLE IF EXISTS ad_status_event;
DROP TABLE IF EXISTS ad_image;
DROP TABLE IF EXISTS ad;
DROP TABLE IF EXISTS category;

DROP FUNCTION IF EXISTS create_cart_for_new_user();
DROP FUNCTION IF EXISTS check_message_sender();
DROP FUNCTION IF EXISTS set_conversation_seller();
DROP FUNCTION IF EXISTS log_ad_status_change();
DROP FUNCTION IF EXISTS log_price_change();
DROP FUNCTION IF EXISTS set_updated_at();

DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_rating_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_type_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_nickname_key;
