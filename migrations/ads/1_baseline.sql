-- +goose Up
-- Схема сервиса ads: итог legacy-миграций 1-14 для объявлений, заказов, отзывов и адресов.
-- Колонки с ID пользователей (ad.user_id, orders.buyer_id, review.seller_id и т.д.) принадлежат auth,
-- поэтому без внешних ключей: данные пользователей ads получает по gRPC.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE category (
    category_id      integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id        integer REFERENCES category(category_id) ON DELETE SET NULL,
    name             varchar(100) NOT NULL,
    slug             varchar(120) NOT NULL UNIQUE,
    delivery_allowed boolean NOT NULL DEFAULT true,
    CHECK (parent_id IS NULL OR parent_id <> category_id)
);

INSERT INTO category (name, slug, delivery_allowed) VALUES
    ('Авто', 'auto', false),
    ('Недвижимость', 'realty', false),
    ('Жильё для путешествий', 'rent', false),
    ('Для дома и дачи', 'home', true),
    ('Запчасти', 'parts', true),
    ('Услуги', 'services', false),
    ('Электроника', 'electronics', true),
    ('Работа и подработка', 'jobs', false),
    ('Бизнес 360', 'biznes', true),
    ('Одежда, обувь, аксессуары', 'clothes', true),
    ('Животные', 'pets', true),
    ('Детские товары', 'kids', true),
    ('Красота и здоровье', 'beauty', true),
    ('Спорт и отдых', 'sport', true),
    ('Книги и журналы', 'books', true),
    ('Хобби и фото', 'hobby', true)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO category (parent_id, slug, name, delivery_allowed)
SELECT parent.category_id, child.slug, child.name, parent.delivery_allowed
FROM (VALUES
    ('auto', 'auto-cars', 'Легковые автомобили'),
    ('auto', 'auto-motorcycles', 'Мотоциклы и мототехника'),
    ('auto', 'auto-trucks', 'Грузовики и спецтехника'),
    ('auto', 'auto-water-transport', 'Водный транспорт'),
    ('auto', 'auto-tires', 'Шины и диски'),
    ('realty', 'realty-apartments', 'Квартиры'),
    ('realty', 'realty-rooms', 'Комнаты'),
    ('realty', 'realty-houses', 'Дома и дачи'),
    ('realty', 'realty-land', 'Земельные участки'),
    ('realty', 'realty-commercial', 'Коммерческая недвижимость'),
    ('rent', 'rent-apartments', 'Квартиры посуточно'),
    ('rent', 'rent-houses', 'Дома и коттеджи'),
    ('rent', 'rent-hotels', 'Гостиницы'),
    ('rent', 'rent-hostels', 'Хостелы'),
    ('rent', 'rent-glamping', 'Глэмпинги'),
    ('home', 'home-furniture', 'Мебель'),
    ('home', 'home-appliances', 'Бытовая техника'),
    ('home', 'home-kitchen', 'Посуда и кухня'),
    ('home', 'home-renovation', 'Ремонт и строительство'),
    ('home', 'home-plants', 'Растения'),
    ('parts', 'parts-cars', 'Для автомобилей'),
    ('parts', 'parts-motorcycles', 'Для мототехники'),
    ('parts', 'parts-oils', 'Масла и автохимия'),
    ('parts', 'parts-batteries', 'Аккумуляторы'),
    ('parts', 'parts-tools', 'Инструменты'),
    ('services', 'services-renovation', 'Ремонт и отделка'),
    ('services', 'services-moving', 'Перевозки и грузчики'),
    ('services', 'services-courses', 'Обучение и курсы'),
    ('services', 'services-beauty', 'Красота'),
    ('services', 'services-cleaning', 'Уборка'),
    ('electronics', 'electronics-phones', 'Телефоны'),
    ('electronics', 'electronics-laptops', 'Ноутбуки'),
    ('electronics', 'electronics-audio-video', 'Аудио и видео'),
    ('electronics', 'electronics-gaming', 'Игры и приставки'),
    ('electronics', 'electronics-cameras', 'Фототехника'),
    ('jobs', 'jobs-vacancies', 'Вакансии'),
    ('jobs', 'jobs-resumes', 'Резюме'),
    ('jobs', 'jobs-part-time', 'Подработка'),
    ('jobs', 'jobs-remote', 'Удалённая работа'),
    ('jobs', 'jobs-internships', 'Стажировки'),
    ('biznes', 'biznes-businesses', 'Готовый бизнес'),
    ('biznes', 'biznes-equipment', 'Оборудование'),
    ('biznes', 'biznes-franchises', 'Франшизы'),
    ('biznes', 'biznes-premises', 'Помещения'),
    ('biznes', 'biznes-investments', 'Инвестиции'),
    ('clothes', 'clothes-women', 'Женская одежда'),
    ('clothes', 'clothes-men', 'Мужская одежда'),
    ('clothes', 'clothes-shoes', 'Обувь'),
    ('clothes', 'clothes-bags', 'Сумки и рюкзаки'),
    ('clothes', 'clothes-jewelry', 'Украшения'),
    ('pets', 'pets-dogs', 'Собаки'),
    ('pets', 'pets-cats', 'Кошки'),
    ('pets', 'pets-birds', 'Птицы'),
    ('pets', 'pets-aquariums', 'Аквариум'),
    ('pets', 'pets-supplies', 'Товары для питомцев'),
    ('kids', 'kids-clothes', 'Детская одежда'),
    ('kids', 'kids-toys', 'Игрушки'),
    ('kids', 'kids-strollers', 'Коляски'),
    ('kids', 'kids-car-seats', 'Автокресла'),
    ('kids', 'kids-food', 'Питание'),
    ('beauty', 'beauty-cosmetics', 'Косметика'),
    ('beauty', 'beauty-perfume', 'Парфюмерия'),
    ('beauty', 'beauty-care', 'Уход за собой'),
    ('beauty', 'beauty-accessories', 'Приборы и аксессуары'),
    ('beauty', 'beauty-health', 'Здоровье'),
    ('sport', 'sport-bikes', 'Велосипеды'),
    ('sport', 'sport-fitness', 'Тренажёры'),
    ('sport', 'sport-camping', 'Туризм'),
    ('sport', 'sport-winter', 'Зимние виды спорта'),
    ('sport', 'sport-scooters', 'Самокаты'),
    ('books', 'books-fiction', 'Художественная литература'),
    ('books', 'books-textbooks', 'Учебники'),
    ('books', 'books-kids', 'Детские книги'),
    ('books', 'books-magazines', 'Журналы'),
    ('books', 'books-comics', 'Комиксы'),
    ('hobby', 'hobby-instruments', 'Музыкальные инструменты'),
    ('hobby', 'hobby-cameras', 'Фототехника'),
    ('hobby', 'hobby-collectibles', 'Коллекционирование'),
    ('hobby', 'hobby-crafts', 'Рукоделие'),
    ('hobby', 'hobby-board-games', 'Настольные игры')
) AS child (parent_slug, slug, name)
JOIN category parent ON parent.slug = child.parent_slug
ON CONFLICT (slug) DO NOTHING;

CREATE TABLE ad (
    id                       integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title                    varchar(200) NOT NULL,
    description              text,
    price                    bigint CHECK (price >= 0),
    user_id                  integer NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    status                   varchar(20) NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold')),
    category_id              integer REFERENCES category(category_id) ON DELETE SET NULL,
    type_ad                  varchar(20) NOT NULL DEFAULT 'goods' CHECK (type_ad IN ('goods', 'service')),
    city                     varchar(100),
    has_delivery             boolean NOT NULL DEFAULT false,
    is_free                  boolean NOT NULL DEFAULT false,
    is_negotiable            boolean NOT NULL DEFAULT false,
    condition                varchar(20) CHECK (condition IN ('new', 'used_good', 'used_repair')),
    address                  varchar(255),
    close_reason             varchar(20)
                             CHECK (close_reason IN ('sold_on_goget', 'sold_elsewhere', 'changed_mind', 'other')),
    published_at             timestamptz,
    moderation_revision      bigint NOT NULL DEFAULT 1,
    rejection_reason         varchar(500),
    moderation_requested_at  timestamptz
);

CREATE INDEX idx_ad_user_id ON ad (user_id);
CREATE INDEX idx_ad_category_id ON ad (category_id);
CREATE INDEX idx_ad_status_category_city ON ad (status, category_id, city);
CREATE INDEX idx_ad_user_status ON ad (user_id, status);
CREATE INDEX ad_moderation_fifo ON ad (moderation_requested_at, id) WHERE status = 'moderation';

CREATE TRIGGER trg_ad_updated_at
BEFORE UPDATE ON ad
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE ad_image (
    image_id   integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id      integer REFERENCES ad(id) ON DELETE CASCADE,
    url        text NOT NULL,
    position   integer NOT NULL DEFAULT 0 CHECK (position >= 0),
    alt_text   text,
    created_at timestamptz NOT NULL DEFAULT now(),
    user_id    integer NOT NULL,
    UNIQUE (ad_id, position)
);

CREATE INDEX idx_ad_image_ad_id ON ad_image (ad_id);
CREATE INDEX idx_ad_image_unattached ON ad_image (created_at) WHERE ad_id IS NULL;

CREATE TABLE cart (
    id         integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    integer NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cart_item (
    id        integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cart_id   integer NOT NULL REFERENCES cart(id) ON DELETE CASCADE,
    ad_id     integer NOT NULL,
    seller_id integer NOT NULL,
    UNIQUE (cart_id, ad_id)
);

CREATE TABLE favorites (
    id      integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id integer NOT NULL,
    ad_id   integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    UNIQUE (user_id, ad_id)
);

CREATE TABLE ad_status_event (
    stat_id    integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id      integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    actor_id   integer,
    status     varchar(20) NOT NULL
               CHECK (status IN ('draft', 'moderation', 'rejected', 'active', 'closed', 'sold')),
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE price_history (
    prh_id     integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id      integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    price      bigint NOT NULL CHECK (price >= 0),
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE conversation (
    room_id    integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id      integer REFERENCES ad(id) ON DELETE SET NULL,
    buyer_id   integer NOT NULL,
    seller_id  integer,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE message (
    message_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sender_id  integer NOT NULL,
    room_id    integer NOT NULL REFERENCES conversation(room_id) ON DELETE CASCADE,
    text       text NOT NULL,
    sent_at    timestamptz NOT NULL DEFAULT now(),
    status     varchar(20) NOT NULL DEFAULT 'sent' CHECK (status IN ('sent', 'delivered', 'read'))
);

CREATE TABLE views (
    views_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id  integer,
    ad_id    integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    view_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE review (
    rew_id        integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       integer NOT NULL,
    ad_id         integer REFERENCES ad(id) ON DELETE SET NULL,
    text          text,
    rating        numeric(3,2) NOT NULL CHECK (rating >= 1 AND rating <= 5),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    seller_id     integer NOT NULL,
    listing_title varchar(200),
    UNIQUE (user_id, ad_id)
);

CREATE INDEX idx_review_seller_id ON review (seller_id, created_at);
CREATE INDEX idx_review_ad_id ON review (ad_id);

CREATE TRIGGER trg_review_updated_at
BEFORE UPDATE ON review
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

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
    seller_id   integer NOT NULL,
    reporter_id integer NOT NULL,
    reason      varchar(20) NOT NULL CHECK (reason IN ('fraud', 'prohibited', 'rude', 'other')),
    comment     varchar(500) NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (seller_id <> reporter_id)
);

CREATE INDEX idx_seller_report_seller_id ON seller_report (seller_id, created_at);

CREATE TABLE promotion (
    prom_id         integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id           integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    when_paid       timestamptz NOT NULL DEFAULT now(),
    end_date        timestamptz NOT NULL CHECK (end_date > when_paid),
    promotion_level integer NOT NULL CHECK (promotion_level >= 0)
);

CREATE TABLE ad_reaction (
    like_id  integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id  integer NOT NULL,
    ad_id    integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    is_like  boolean NOT NULL DEFAULT false,
    UNIQUE (user_id, ad_id)
);

CREATE TABLE address (
    address_id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    integer NOT NULL,
    street     varchar(200) NOT NULL,
    house      varchar(50) NOT NULL,
    comment    varchar(200) NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    city       varchar(100) NOT NULL DEFAULT '',
    apartment  varchar(20) NOT NULL DEFAULT ''
);

CREATE INDEX idx_address_user_id ON address (user_id, created_at);
CREATE UNIQUE INDEX uq_address_default ON address (user_id) WHERE is_default;

CREATE TABLE orders (
    id                 integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    buyer_id           integer NOT NULL,
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
    address_text       varchar(600),
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
    seller_id integer NOT NULL,
    method    varchar(20) NOT NULL CHECK (method IN ('courier', 'pickup_point', 'self_pickup')),
    price     bigint NOT NULL CHECK (price >= 0),
    PRIMARY KEY (order_id, seller_id)
);

CREATE TABLE order_item (
    id        integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id  integer NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    ad_id     integer REFERENCES ad(id) ON DELETE SET NULL,
    seller_id integer NOT NULL,
    title     varchar(200) NOT NULL,
    price     bigint NOT NULL CHECK (price >= 0),
    image_url varchar(255)
);

CREATE INDEX idx_order_item_order_id ON order_item (order_id);
CREATE INDEX idx_order_item_ad_id ON order_item (ad_id);

CREATE TABLE moderation_decision (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ad_id        integer NOT NULL REFERENCES ad(id) ON DELETE CASCADE,
    moderator_id integer NOT NULL,
    revision     bigint NOT NULL,
    status       varchar(20) NOT NULL CHECK (status IN ('active', 'rejected')),
    reason       varchar(500),
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'active' AND reason IS NULL)
           OR (status = 'rejected' AND reason IS NOT NULL AND length(trim(reason)) > 0))
);

CREATE UNIQUE INDEX moderation_decision_ad_revision ON moderation_decision (ad_id, revision);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_price_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.price IS NULL THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'INSERT' OR NEW.price IS DISTINCT FROM OLD.price THEN
        INSERT INTO price_history (ad_id, price, changed_at)
        VALUES (NEW.id, NEW.price, now());
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_ad_price_history
AFTER INSERT OR UPDATE OF price ON ad
FOR EACH ROW EXECUTE FUNCTION log_price_change();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_conversation_seller()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.seller_id IS NULL AND NEW.ad_id IS NOT NULL THEN
        SELECT user_id INTO NEW.seller_id FROM ad WHERE id = NEW.ad_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_conversation_set_seller
BEFORE INSERT ON conversation
FOR EACH ROW EXECUTE FUNCTION set_conversation_seller();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_message_sender()
RETURNS TRIGGER AS $$
DECLARE
    v_buyer_id  INT;
    v_seller_id INT;
BEGIN
    SELECT buyer_id, seller_id
      INTO v_buyer_id, v_seller_id
      FROM conversation
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
-- +goose StatementEnd

CREATE TRIGGER trg_message_sender_check
BEFORE INSERT OR UPDATE OF sender_id, room_id ON message
FOR EACH ROW EXECUTE FUNCTION check_message_sender();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_ad_status_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO ad_status_event (ad_id, actor_id, status, changed_at)
        VALUES (NEW.id, NEW.user_id, NEW.status, now());
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE' AND NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO ad_status_event (ad_id, actor_id, status, changed_at)
        VALUES (NEW.id, NEW.user_id, NEW.status, now());
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_ad_status_event
AFTER INSERT OR UPDATE OF status ON ad
FOR EACH ROW EXECUTE FUNCTION log_ad_status_change();

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

-- +goose Down
DROP TABLE moderation_decision, order_item, order_seller, orders, address, ad_reaction, promotion,
    seller_report, review_image, review, views, message, conversation, price_history,
    ad_status_event, favorites, cart_item, cart, ad_image, ad, category;
DROP FUNCTION track_ad_moderation();
DROP FUNCTION set_review_seller();
DROP FUNCTION log_ad_status_change();
DROP FUNCTION check_message_sender();
DROP FUNCTION set_conversation_seller();
DROP FUNCTION log_price_change();
DROP FUNCTION set_updated_at();
