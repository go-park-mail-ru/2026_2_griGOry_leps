#!/usr/bin/env bash
# Одноразовый перенос данных из общей базы монолита в базы сервисов auth и ads.
# Исходная база не изменяется. Запускать перед activate.sh нового релиза.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)
# shellcheck source=split-db-tables.sh
source "$here/split-db-tables.sh"

backup_dir=${BACKUP_DIR:-/var/backups/goget}
lock_file=${LOCK_FILE:-/run/lock/goget-split-db.lock}
legacy=gogetbackend.service
services=(goget-gateway.service goget-backend@auth.service goget-backend@ads.service goget-backend@search.service)

if [[ -z ${SOURCE_DATABASE_URL:-} || -z ${AUTH_DATABASE_URL:-} || -z ${ADS_DATABASE_URL:-} ]]; then
    echo "Usage: SOURCE_DATABASE_URL=... AUTH_DATABASE_URL=... ADS_DATABASE_URL=... [SKIP_SERVICES=1] $0" >&2
    exit 2
fi
if [[ ${SKIP_SERVICES:-0} != 1 ]] && (( EUID != 0 )); then
    echo "Run as root to stop and start the services, or set SKIP_SERVICES=1" >&2
    exit 2
fi
for tool in psql pg_dump goose; do
    command -v "$tool" >/dev/null || { echo "Missing tool: $tool" >&2; exit 2; }
done

exec 9>"$lock_file"
flock -n 9 || { echo "Another database split is running" >&2; exit 1; }

q() {
    local url=$1
    shift
    psql --no-psqlrc --quiet --tuples-only --no-align -v ON_ERROR_STOP=1 "$url" "$@"
}

mask() { sed -E 's#(://[^:/@]+:)[^@]*@#\1***@#' <<<"$1"; }

# Число строк во всех таблицах, кроме служебных таблиц goose и справочников city/category:
# их заполняют baseline-миграции, а перед копированием скрипт очищает их.
total_rows() {
    local url=$1 table total=0
    while IFS= read -r table; do
        [[ -n $table ]] || continue
        total=$((total + $(q "$url" -c "SELECT count(*) FROM public.\"$table\"")))
    done < <(q "$url" -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name NOT LIKE 'goose%' AND table_name NOT IN ('city', 'category')")
    echo "$total"
}

truncate_all() {
    local url=$1 tables
    tables=$(q "$url" -c "SELECT string_agg(format('public.%I', table_name), ', ') FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name NOT LIKE 'goose%'")
    if [[ -n $tables ]]; then q "$url" -c "TRUNCATE $tables RESTART IDENTITY CASCADE"; fi
}

table_counts() {
    local url=$1 table
    shift
    for table in "$@"; do
        printf '%s %s\n' "$table" "$(q "$url" -c "SELECT count(*) FROM public.\"$table\"")"
    done
}

# Копирует данные таблиц из исходной базы; триггеры не срабатывают, поэтому история и ревизии не дублируются.
copy_tables() {
    local target=$1 table
    shift
    local args=()
    for table in "$@"; do args+=(--table="public.$table"); done
    {
        echo "SET session_replication_role = replica;"
        pg_dump --data-only --no-owner "${args[@]}" "$SOURCE_DATABASE_URL"
    } | psql --no-psqlrc --quiet -v ON_ERROR_STOP=1 --single-transaction --output=/dev/null "$target"
}

# Переносит состояние последовательностей из исходной базы: ID удалённых строк повторно не выдаются.
# max(id) в таблице служит нижней границей на случай, если последовательность отстаёт; пустая таблица
# с нетронутой последовательностью даёт следующий ID 1.
copy_sequences() {
    local target=$1 table column src_seq last called
    shift
    for table in "$@"; do
        while IFS= read -r column; do
            [[ -n $column ]] || continue
            src_seq=$(q "$SOURCE_DATABASE_URL" -c "SELECT pg_get_serial_sequence('public.$table', '$column')")
            IFS='|' read -r last called < <(q "$SOURCE_DATABASE_URL" -c "SELECT last_value, is_called FROM $src_seq")
            [[ $called == t ]] || last=0
            q "$target" -c "
                SELECT CASE WHEN v = 0 THEN setval(s, 1, false) ELSE setval(s, v, true) END
                FROM (SELECT pg_get_serial_sequence('public.$table', '$column') AS s,
                             GREATEST($last, COALESCE((SELECT max(\"$column\") FROM public.\"$table\"), 0)) AS v) x" >/dev/null
        done < <(q "$target" -c "SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '$table' AND is_identity = 'YES'")
    done
}

# 1. Предпроверки: ничего не останавливаем и не меняем, пока они не пройдены.
for target in "$SOURCE_DATABASE_URL" "$AUTH_DATABASE_URL" "$ADS_DATABASE_URL"; do
    if ! q "$target" -c "SELECT 1" >/dev/null; then
        echo "Cannot connect to $(mask "$target")" >&2
        exit 1
    fi
done
# Копирование отключает триггеры через session_replication_role: нужны права суперпользователя.
for target in "$AUTH_DATABASE_URL" "$ADS_DATABASE_URL"; do
    if ! q "$target" -c "SET session_replication_role = replica" >/dev/null; then
        echo "The role for $(mask "$target") cannot set session_replication_role (superuser required)" >&2
        exit 1
    fi
done
version=$(q "$SOURCE_DATABASE_URL" -c "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied")
if [[ $version != 14 ]]; then
    echo "Source database is at goose version $version, expected 14" >&2
    exit 1
fi
for target in "$AUTH_DATABASE_URL" "$ADS_DATABASE_URL"; do
    if [[ $(total_rows "$target") != 0 ]]; then
        echo "Target database $(mask "$target") already contains data; nothing was changed" >&2
        exit 1
    fi
done

# 2. Остановка записи. Обработчик восстановления ставится до первой остановки: частично остановленные
# службы тоже запускаются обратно, а ошибка systemctl не оставляет приложение выключенным.
legacy_running=false
services_stopped=false

restore() {
    local result=$1
    trap - EXIT
    trap '' HUP INT TERM
    set +e
    echo "Split failed. Clearing the target databases." >&2
    truncate_all "$AUTH_DATABASE_URL" || echo "Could not clear the auth database" >&2
    truncate_all "$ADS_DATABASE_URL" || echo "Could not clear the ads database" >&2
    if $services_stopped; then
        if $legacy_running; then
            systemctl start "$legacy" || echo "Could not start $legacy" >&2
        else
            systemctl start "${services[@]}" || echo "Could not start all services" >&2
        fi
    fi
    exit "$result"
}
trap 'restore "$?"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ ${SKIP_SERVICES:-0} != 1 ]]; then
    if systemctl is-active --quiet "$legacy"; then legacy_running=true; fi
    services_stopped=true
    systemctl stop "${services[@]}"
    if $legacy_running; then systemctl stop "$legacy"; fi
fi

# 3. Бэкап.
mkdir -p "$backup_dir"
backup="$backup_dir/goget-$(date +%Y%m%d-%H%M%S).dump"
pg_dump -Fc --file="$backup" "$SOURCE_DATABASE_URL"
echo "Backup: $backup"

# 4. Схемы сервисов.
goose -dir "$repo/migrations/auth" -table goose_auth postgres "$AUTH_DATABASE_URL" up
goose -dir "$repo/migrations/ads" -table goose_ads postgres "$ADS_DATABASE_URL" up
# Справочники из baseline (города, категории) заменяются строками исходной базы.
truncate_all "$AUTH_DATABASE_URL"
truncate_all "$ADS_DATABASE_URL"

# 5. Данные.
copy_tables "$AUTH_DATABASE_URL" "${AUTH_TABLES[@]}"
copy_tables "$ADS_DATABASE_URL" "${ADS_TABLES[@]}"
copy_sequences "$AUTH_DATABASE_URL" "${AUTH_TABLES[@]}"
copy_sequences "$ADS_DATABASE_URL" "${ADS_TABLES[@]}"

# 6. Сверка.
if ! diff <(table_counts "$SOURCE_DATABASE_URL" "${AUTH_TABLES[@]}" "${ADS_TABLES[@]}") \
          <(table_counts "$AUTH_DATABASE_URL" "${AUTH_TABLES[@]}"; table_counts "$ADS_DATABASE_URL" "${ADS_TABLES[@]}"); then
    echo "Row counts differ from the source database" >&2
    exit 1
fi
q "$AUTH_DATABASE_URL" -f "$here/split-db-check.sql"
q "$ADS_DATABASE_URL" -f "$here/split-db-check.sql"
ids_dir=$(mktemp -d)
q "$ADS_DATABASE_URL" -f "$here/split-db-user-ids.sql" >"$ids_dir/ads"
q "$AUTH_DATABASE_URL" -c "SELECT user_id FROM user_account" >"$ids_dir/auth"
missing=$(comm -23 <(sort -u "$ids_dir/ads") <(sort -u "$ids_dir/auth"))
rm -rf "$ids_dir"
if [[ -n $missing ]]; then
    echo "ads references users that are missing in auth: $(tr '\n' ' ' <<<"$missing")" >&2
    exit 1
fi

# 7. Готово: сервисы остаются остановленными, оператор переключает .env и запускает activate.sh.
trap - EXIT HUP INT TERM
echo "Data copied and verified. Add to .env, then run activate.sh:"
echo "AUTH_DATABASE_URL=$(mask "$AUTH_DATABASE_URL")"
echo "ADS_DATABASE_URL=$(mask "$ADS_DATABASE_URL")"
