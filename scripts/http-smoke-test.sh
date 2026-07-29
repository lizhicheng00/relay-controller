#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://localhost:8443}"
API_BASE="$BASE_URL/open-api-inner/v1/relay-controller"
NAMESPACE="${NAMESPACE:-ns-user-001}"
TUNNEL_ID="${TUNNEL_ID:-aaaadysa}"
CLUSTER_ID="${CLUSTER_ID:-cluster-a}"
: "${TLS_CA_CERT:?set TLS_CA_CERT to the server CA certificate}"
: "${TLS_CLIENT_CERT:?set TLS_CLIENT_CERT to the client certificate}"
: "${TLS_CLIENT_KEY:?set TLS_CLIENT_KEY to the client private key}"

CURL_TLS=(--cacert "$TLS_CA_CERT" --cert "$TLS_CLIENT_CERT" --key "$TLS_CLIENT_KEY")

request() {
  local name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local namespace_header="${5:-}"
  local response http_code content
  local headers=(-H "Content-Type: application/json")

  if [[ -n "$namespace_header" ]]; then
    headers+=(-H "X-Namespace: $NAMESPACE")
  fi

  if [[ -n "$body" ]]; then
    response=$(curl -sS "${CURL_TLS[@]}" -X "$method" "$url" "${headers[@]}" -d "$body" -w $'\nHTTP_STATUS:%{http_code}')
  else
    response=$(curl -sS "${CURL_TLS[@]}" -X "$method" "$url" "${headers[@]}" -w $'\nHTTP_STATUS:%{http_code}')
  fi

  http_code=$(printf '%s' "$response" | awk -F: '/HTTP_STATUS/{print $2}')
  content=$(printf '%s' "$response" | sed '/HTTP_STATUS:/d' | tr '\n' ' ' | cut -c 1-520)
  printf '%-36s %-4s %s\n' "$name" "$http_code" "$content"
}

request "01 create missing namespace" POST "$API_BASE/tunnels" "{\"name\":\"dev\",\"clusterId\":\"$CLUSTER_ID\",\"type\":\"bridge\"}"
request "02 create invalid type" POST "$API_BASE/tunnels" "{\"name\":\"dev\",\"clusterId\":\"$CLUSTER_ID\",\"type\":\"default\"}" yes
request "03 create bridge" POST "$API_BASE/tunnels" "{\"name\":\"dev\",\"clusterId\":\"$CLUSTER_ID\",\"expiration\":24,\"type\":\"bridge\"}" yes
request "04 list tunnels" GET "$API_BASE/tunnels?clusterId=$CLUSTER_ID" "" yes
request "05 tunnel detail" GET "$API_BASE/tunnels/$TUNNEL_ID" "" yes
request "06 update tunnel env" PUT "$API_BASE/tunnels/$TUNNEL_ID" "{\"type\":\"env\"}" yes
request "07 metering" POST "$API_BASE/clusters/$CLUSTER_ID/metering" "{\"tunnelCode\":123456,\"tunnelId\":\"$TUNNEL_ID\",\"usage\":1024}"
request "08 issue host token" POST "$API_BASE/tunnels/$TUNNEL_ID/token?scope=host" "" yes
request "09 issue cookie token" POST "$API_BASE/tunnels/$TUNNEL_ID/token?scope=connect&forCookies=true" "" yes
request "10 limits" GET "$API_BASE/limits" "" yes
request "11 create port" POST "$API_BASE/tunnels/$TUNNEL_ID/ports" "{\"port\":8080,\"protocol\":\"auto\",\"allowAnonymous\":false}" yes
request "12 list ports" GET "$API_BASE/tunnels/$TUNNEL_ID/ports" "" yes
request "13 get port" GET "$API_BASE/tunnels/$TUNNEL_ID/ports/8080" "" yes
request "14 update port" PUT "$API_BASE/tunnels/$TUNNEL_ID/ports/8080" "{\"allowAnonymous\":true}" yes
request "15 gateway port policy" GET "$API_BASE/clusters/$CLUSTER_ID/tunnels/$TUNNEL_ID/ports/8080"
request "16 delete port" DELETE "$API_BASE/tunnels/$TUNNEL_ID/ports/8080" "" yes
request "17 delete tunnel" DELETE "$API_BASE/tunnels/$TUNNEL_ID" "" yes
request "18 delete tunnels" DELETE "$API_BASE/tunnels" "" yes
request "19 openapi yaml" GET "$BASE_URL/openapi.yaml"
