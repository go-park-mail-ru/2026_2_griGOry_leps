-- +goose Up
CREATE TABLE onlineconf_entries (
    id      integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scope   varchar(20) NOT NULL
            CHECK (scope IN ('frontend', 'auth', 'ads', 'search')),
    name    varchar(255) NOT NULL CHECK (name ~ '[^[:space:]]'),
    kind    TEXT NOT NULL CHECK (kind IN ('config', 'flag')),
    value   jsonb NOT NULL,
    UNIQUE (scope, name),
    CHECK (kind <> 'flag' OR jsonb_typeof(value) = 'boolean')
);

-- +goose Down
DROP TABLE onlineconf_entries;
