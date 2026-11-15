#!/bin/sh
set -eu

# Run the Gitaly daemon itself through this wrapper, so every Git subprocess
# inherits direct egress even when the surrounding deployment injects proxies.
# NO_PROXY also bypasses an http.proxy value supplied through Git configuration.
unset http_proxy HTTP_PROXY https_proxy HTTPS_PROXY all_proxy ALL_PROXY
export no_proxy='*'
export NO_PROXY='*'

if [ "$#" -eq 0 ]; then
    echo 'usage: direct-egress.sh <gitaly startup command> [args...]' >&2
    exit 1
fi
exec "$@"
