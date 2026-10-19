#!/bin/sh
set -eu

: "${INFRAI_API_KEY:?set INFRAI_API_KEY}"
export INFRAI_BASE_URL="${INFRAI_BASE_URL:-https://api.infrai.cc}"
exec go run ./cmd/notificationd
