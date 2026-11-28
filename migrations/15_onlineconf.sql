-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION onlineconf_valid_user_ids(ids jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT CASE WHEN jsonb_typeof(ids) = 'array' THEN NOT EXISTS (
        SELECT 1 FROM jsonb_array_elements(ids) AS elements(id)
        WHERE CASE WHEN jsonb_typeof(id) = 'number' THEN
            id::numeric NOT BETWEEN 1 AND 2147483647 OR trunc(id::numeric) <> id::numeric
        ELSE true END
    ) ELSE false END
$$;
-- +goose StatementEnd

CREATE TABLE onlineconf_entries (
    id      integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scope   varchar(20) NOT NULL
            CHECK (scope IN ('frontend', 'auth', 'ads', 'search')),
    name    varchar(255) NOT NULL CHECK (name ~ '[^[:space:]]'),
    kind    TEXT NOT NULL CHECK (kind IN ('config', 'flag')),
    value   jsonb NOT NULL,
    UNIQUE (scope, name),
    CONSTRAINT onlineconf_flag_value_check CHECK (
        kind <> 'flag' OR jsonb_typeof(value) = 'boolean' OR (
            jsonb_typeof(value) = 'object'
            AND value ?& ARRAY['rollout_percent', 'included_user_ids', 'excluded_user_ids']
            AND value - ARRAY['rollout_percent', 'included_user_ids', 'excluded_user_ids'] = '{}'::jsonb
            AND CASE WHEN jsonb_typeof(value->'rollout_percent') = 'number' THEN
                (value->>'rollout_percent')::numeric BETWEEN 0 AND 100
                AND trunc((value->>'rollout_percent')::numeric) = (value->>'rollout_percent')::numeric
            ELSE false END
            AND onlineconf_valid_user_ids(value->'included_user_ids')
            AND onlineconf_valid_user_ids(value->'excluded_user_ids')
        )
    ),
    CONSTRAINT onlineconf_search_length_check CHECK (
        scope <> 'search' OR name <> 'max_query_length' OR (
            kind = 'config' AND jsonb_typeof(value) = 'number'
            AND value::text ~ '^[1-9][0-9]*$'
            AND value <= '9223372036854775807'::jsonb
        )
    ),
    CONSTRAINT onlineconf_rollout_integers_check CHECK (
        kind <> 'flag' OR jsonb_typeof(value) <> 'object' OR (
            (value->>'rollout_percent') ~ '^[0-9]+$'
            AND (value->>'included_user_ids') !~ '[.]'
            AND (value->>'excluded_user_ids') !~ '[.]'
        )
    )
);

-- +goose Down
DROP TABLE onlineconf_entries;
DROP FUNCTION onlineconf_valid_user_ids(jsonb);
