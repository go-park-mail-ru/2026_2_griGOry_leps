-- Для каждой identity-колонки следующий ID должен быть больше максимального существующего.
DO $$
DECLARE
    r record;
    max_id bigint;
    last bigint;
    seq text;
BEGIN
    FOR r IN
        SELECT table_name, column_name FROM information_schema.columns
        WHERE table_schema = 'public' AND is_identity = 'YES' AND table_name NOT LIKE 'goose%'
    LOOP
        seq := pg_get_serial_sequence(format('public.%I', r.table_name), r.column_name);
        EXECUTE format('SELECT max(%I) FROM public.%I', r.column_name, r.table_name) INTO max_id;
        EXECUTE format('SELECT last_value FROM %s', seq) INTO last;
        IF max_id IS NOT NULL AND last < max_id THEN
            RAISE EXCEPTION 'sequence % is behind %.%: % < %', seq, r.table_name, r.column_name, last, max_id;
        END IF;
    END LOOP;
END $$;
