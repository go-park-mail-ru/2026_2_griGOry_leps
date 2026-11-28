#!/usr/bin/env bash
set -euo pipefail

auth=${AUTH_URL:-http://127.0.0.1:8081}
ads=${ADS_URL:-http://127.0.0.1:8082}
search=${SEARCH_URL:-http://127.0.0.1:8083}
onlineconf=${ONLINECONF_URL:-http://127.0.0.1:8084}
gateway=${GATEWAY_URL:-http://127.0.0.1:8000}
services=(auth ads search onlineconf)
legacy_release=false
if [[ $# == 1 && $1 == --legacy ]]; then
    legacy_release=true
    services=(auth ads search)
elif (( $# != 0 )); then
    echo "Usage: $0 [--legacy]" >&2
    exit 2
fi

get() {
    curl --silent --show-error --fail --connect-timeout 2 --max-time 5 "$1"
}

status() {
    curl --silent --show-error --connect-timeout 2 --max-time 5 \
        --output /dev/null --write-out '%{http_code}' "$@"
}

check() {
    local service url code
    for service in "${services[@]}"; do
        case "$service" in
            auth) url=$auth ;;
            ads) url=$ads ;;
            search) url=$search ;;
            onlineconf) url=$onlineconf ;;
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
    if ! code=$(status --cookie "session_id=$(printf '%064d' 0)" "$gateway/api/cart") || [[ $code != 401 ]]; then
        echo "gateway -> ads -> auth gRPC: session check returned ${code:-no response}" >&2
        return 1
    fi
    if ! $legacy_release; then
        if ! code=$(status --cookie "session_id=$(printf '%064d' 0)" "$gateway/api/onlineconf/admin/me") || [[ $code != 401 ]]; then
            echo "gateway -> onlineconf -> auth gRPC: admin check returned ${code:-no response}" >&2
            return 1
        fi
    fi
    if ! code=$(status "$gateway/api/search?q=g") ||
        [[ $code != 200 && ! ( $code == 501 && ${ALLOW_SEARCH_STUB:-0} == 1 ) ]]; then
        echo "gateway -> search -> ads gRPC: search check returned ${code:-no response}" >&2
        return 1
    fi
}

for ((attempt=1; attempt<=10; attempt++)); do
    if check; then
        if $legacy_release; then
            echo "auth, ads, search and gateway are ready (legacy release)"
        else
            echo "auth, ads, search, onlineconf and gateway are ready"
        fi
        exit 0
    fi
    if (( attempt < 10 )); then sleep 1; fi
done
exit 1
