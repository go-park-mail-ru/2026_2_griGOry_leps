#!/usr/bin/env bash
set -euo pipefail

auth=${AUTH_URL:-http://127.0.0.1:8081}
ads=${ADS_URL:-http://127.0.0.1:8082}
search=${SEARCH_URL:-http://127.0.0.1:8083}
gateway=${GATEWAY_URL:-http://127.0.0.1:8000}

get() {
    curl --silent --show-error --fail --connect-timeout 2 --max-time 5 "$1"
}

status() {
    curl --silent --show-error --connect-timeout 2 --max-time 5 \
        --output /dev/null --write-out '%{http_code}' "$@"
}

check() {
    local service url code
    for service in auth ads search; do
        case "$service" in
            auth) url=$auth ;;
            ads) url=$ads ;;
            search) url=$search ;;
        esac
        if ! get "$url/api/health" | jq -e --arg service "$service" '.status == "ok" and .service == $service' >/dev/null; then
            echo "$service: health check failed" >&2
            return 1
        fi
    done

    if ! get "$gateway/api/csrf" | jq -e '.csrf_token | type == "string" and length > 0' >/dev/null; then
        echo "gateway -> auth: CSRF check failed" >&2
        return 1
    fi
    if ! get "$gateway/api/listings?limit=1" | jq -e '.items | type == "array"' >/dev/null; then
        echo "gateway -> ads -> PostgreSQL: catalog check failed" >&2
        return 1
    fi
    if id=$(get "$gateway/api/listings?limit=1" | jq -r '.items[0].id // empty') && [[ -n $id ]] &&
        ! get "$gateway/api/listings/$id" | jq -e '.seller.first_name | type == "string" and length > 0' >/dev/null; then
        echo "gateway -> ads -> auth gRPC: listing $id has no seller data" >&2
        return 1
    fi
    if ! code=$(status --cookie "session_id=$(printf '%064d' 0)" "$gateway/api/cart") || [[ $code != 401 ]]; then
        echo "gateway -> ads -> auth gRPC: session check returned ${code:-no response}" >&2
        return 1
    fi
    if ! code=$(status "$gateway/api/search?q=goget") ||
        [[ $code != 200 && ! ( $code == 501 && ${ALLOW_SEARCH_STUB:-0} == 1 ) ]]; then
        echo "gateway -> search -> ads gRPC: search check returned ${code:-no response}" >&2
        return 1
    fi
}

for ((attempt=1; attempt<=10; attempt++)); do
    if check; then
        echo "auth, ads, search and gateway are ready"
        exit 0
    fi
    if (( attempt < 10 )); then sleep 1; fi
done
exit 1
