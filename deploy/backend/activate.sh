#!/usr/bin/env bash
set -euo pipefail

root=/opt/goget/backend
legacy=gogetbackend.service
gateway=goget-gateway.service
services=(goget-backend@auth.service goget-backend@ads.service goget-backend@search.service goget-backend@onlineconf.service "$gateway")
all_services=("${services[@]}")
legacy_release=false
if [[ ${1:-} == --legacy ]]; then
    legacy_release=true
    shift
fi
release=${1:-}

if (( EUID != 0 )) || (( $# != 1 )) || [[ ! $release =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
    echo "Usage: sudo $0 [--legacy] RELEASE" >&2
    exit 2
fi

exec 9>/run/lock/goget-backend.lock
flock -n 9 || { echo "Another GoGet activation is running" >&2; exit 1; }

target="$root/releases/$release"
required=(auth ads search gateway)
if $legacy_release; then
    if [[ -e "$target/onlineconf" ]]; then
        echo "Legacy release must not contain onlineconf" >&2
        exit 1
    fi
else
    required+=(onlineconf)
fi
for service in "${required[@]}"; do
    if [[ ! -x "$target/$service" ]]; then
        echo "Missing executable: $target/$service" >&2
        exit 1
    fi
done

select_services() {
    services=(goget-backend@auth.service goget-backend@ads.service goget-backend@search.service)
    health_args=()
    if $1; then
        health_args=(--legacy)
    else
        services+=(goget-backend@onlineconf.service)
    fi
    services+=("$gateway")
}
select_services "$legacy_release"

previous=''
if [[ -L "$root/current" ]]; then
    previous=$(readlink -e "$root/current") || exit 1
    [[ $previous == "$root/releases/"* ]] || { echo "Invalid current release: $previous" >&2; exit 1; }
elif [[ -e "$root/current" ]]; then
    echo "Expected current to be a symlink" >&2
    exit 1
fi

legacy_running=false
legacy_enabled=false
gateway_enabled=false
if systemctl is-active --quiet "$legacy"; then legacy_running=true; fi
if systemctl is-enabled --quiet "$legacy"; then legacy_enabled=true; fi
if systemctl is-enabled --quiet "$gateway"; then gateway_enabled=true; fi

link="$root/.current.$$"
health_pid=''

select_release() {
    ln -s -- "$1" "$link" && mv -Tf -- "$link" "$root/current"
}

rollback() {
    local result=$1
    local rollback_failed=0
    trap - EXIT
    trap '' HUP INT TERM

    if [[ -n $health_pid ]]; then
        kill "$health_pid" 2>/dev/null || true
        wait "$health_pid" 2>/dev/null || true
    fi

    echo "Activation failed. Restoring the previous backend." >&2
    systemctl stop "${all_services[@]}" || rollback_failed=1
    if ! $gateway_enabled; then systemctl disable "$gateway" || rollback_failed=1; fi
    if $legacy_enabled; then systemctl enable "$legacy" || rollback_failed=1; fi
    rm -f -- "$link" || rollback_failed=1
    if [[ -n $previous ]]; then
        select_release "$previous" || rollback_failed=1
    else
        rm -f -- "$root/current" || rollback_failed=1
    fi
    if $legacy_running; then
        systemctl start "$legacy" || rollback_failed=1
        curl --silent --show-error --fail --retry 5 --retry-connrefused --max-time 5 \
            http://127.0.0.1:8000/api/health >/dev/null || rollback_failed=1
    elif [[ -n $previous ]]; then
        if [[ -e "$previous/onlineconf" ]]; then
            select_services false
        else
            select_services true
        fi
        systemctl start "${services[@]}" &&
            ALLOW_SEARCH_STUB=1 "$root/bin/health.sh" "${health_args[@]}" || rollback_failed=1
    fi
    if (( rollback_failed )); then echo "Rollback requires operator attention" >&2; fi
    exit "$result"
}

trap 'rollback "$?"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if $legacy_running; then
    systemctl stop "$legacy"
fi

select_release "$target"
if $legacy_release; then
    systemctl stop goget-backend@onlineconf.service
fi
systemctl restart "${services[@]}"
"$root/bin/health.sh" "${health_args[@]}" &
health_pid=$!
if wait "$health_pid"; then
    health_pid=''
else
    result=$?
    health_pid=''
    exit "$result"
fi
systemctl enable "$gateway"
if $legacy_enabled; then systemctl disable "$legacy"; fi

trap - EXIT HUP INT TERM
echo "GoGet backend release $release is active"
