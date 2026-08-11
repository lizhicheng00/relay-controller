#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://localhost:8443}"
API_BASE="$BASE_URL/open-api-inner/v1/relay-controller"
NAMESPACE="${NAMESPACE:-ns-smoke-$(date +%s)-$$}"
ACCOUNT_NAMESPACE="${ACCOUNT_NAMESPACE:-$NAMESPACE}"
CLUSTER_ID="${CLUSTER_ID:-cn-north-4-bridge}"
PORT="${PORT:-18081}"

CURL_OPTIONS=(-sS)
if [[ "$BASE_URL" == https://* ]]; then
  : "${TLS_CA_CERT:?set TLS_CA_CERT to the server CA certificate}"
  : "${TLS_CLIENT_CERT:?set TLS_CLIENT_CERT to the client certificate}"
  : "${TLS_CLIENT_KEY:?set TLS_CLIENT_KEY to the client private key}"
  CURL_OPTIONS+=(--cacert "$TLS_CA_CERT" --cert "$TLS_CLIENT_CERT" --key "$TLS_CLIENT_KEY")
fi

LAST_BODY=""

request() {
  local name="$1" method="$2" url="$3" expected="$4"
  local body="${5:-}" with_context="${6:-true}"
  local response status args=(-X "$method" "$url" -w $'\n%{http_code}')

  if [[ "$with_context" == true ]]; then
    args+=(-H "X-Namespace: $NAMESPACE" -H "X-Account-Namespace: $ACCOUNT_NAMESPACE")
  fi
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi

  response=$(curl "${CURL_OPTIONS[@]}" "${args[@]}")
  status="${response##*$'\n'}"
  LAST_BODY="${response%$'\n'*}"
  printf '%-30s %s  %.180s\n' "$name" "$status" "$LAST_BODY"
  if [[ "$status" != "$expected" ]]; then
    printf 'expected HTTP %s, got %s\n' "$expected" "$status" >&2
    return 1
  fi
}

request "OpenAPI" GET "$BASE_URL/openapi.yaml" 200 "" false
request "Missing namespace" GET "$API_BASE/tunnels" 401 "" false
request "Create tunnel" POST "$API_BASE/tunnels" 200 \
  "{\"name\":\"smoke\",\"clusterId\":\"$CLUSTER_ID\",\"expiration\":24,\"type\":\"bridge\"}"

TUNNEL_ID=$(printf '%s' "$LAST_BODY" | sed -n 's/.*"tunnelId":"\([^"]*\)".*/\1/p')
if [[ -z "$TUNNEL_ID" ]]; then
  printf 'create response has no tunnelId\n' >&2
  exit 1
fi

request "List tunnels" GET "$API_BASE/tunnels?clusterId=$CLUSTER_ID" 200
request "Tunnel detail" GET "$API_BASE/tunnels/$TUNNEL_ID" 200
request "Update tunnel" PUT "$API_BASE/tunnels/$TUNNEL_ID" 200 '{"description":"updated","type":"env"}'
request "Issue host token" POST "$API_BASE/tunnels/$TUNNEL_ID/token?scope=host" 200
request "Issue connect token" POST "$API_BASE/tunnels/$TUNNEL_ID/token?scope=connect" 200
request "Create port" POST "$API_BASE/tunnels/$TUNNEL_ID/ports" 200 \
  "{\"port\":$PORT,\"protocol\":\"auto\",\"allowAnonymous\":false}"
request "List ports" GET "$API_BASE/tunnels/$TUNNEL_ID/ports" 200
request "Update port" PUT "$API_BASE/tunnels/$TUNNEL_ID/ports/$PORT" 200 '{"allowAnonymous":true}'
request "Limits" GET "$API_BASE/limits" 200
request "Delete port" DELETE "$API_BASE/tunnels/$TUNNEL_ID/ports/$PORT" 200
request "Delete tunnel" DELETE "$API_BASE/tunnels/$TUNNEL_ID" 200
request "Deleted tunnel" GET "$API_BASE/tunnels/$TUNNEL_ID" 404

printf 'smoke test passed\n'
