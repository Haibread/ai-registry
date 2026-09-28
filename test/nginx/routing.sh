#!/usr/bin/env bash
# Runs web/nginx.conf in the web image's nginx base, in front of a stub backend,
# and checks which paths reach the backend and which headers each response gets.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
port="${NGINX_TEST_PORT:-18080}"
name="ai-registry-nginx-test-$$"
backend="${name}-backend"
backend_csp="default-src 'none'"
image="$(awk '$1 == "FROM" && $NF == "runner" { print $2 }' "${repo_root}/web/Dockerfile")"

html="$(mktemp -d)"
chmod 755 "${html}"
echo '<!doctype html><title>spa</title>' >"${html}/index.html"
cat >"${html}/backend.conf" <<CONF
server {
    listen 8080;
    add_header Content-Security-Policy "${backend_csp}" always;
    add_header X-Frame-Options "DENY" always;
    return 200 "backend\n";
}
CONF
chmod 644 "${html}/index.html" "${html}/backend.conf"

cleanup() {
	docker rm -f "${name}" "${backend}" >/dev/null 2>&1 || true
	docker network rm "${name}" >/dev/null 2>&1 || true
	rm -rf "${html}"
}
trap cleanup EXIT

docker network create "${name}" >/dev/null
docker run -d --name "${backend}" --network "${name}" \
	-v "${html}/backend.conf:/etc/nginx/conf.d/default.conf:ro" \
	"${image}" >/dev/null
docker run -d --name "${name}" --network "${name}" -p "127.0.0.1:${port}:8080" \
	-e API_URL="http://${backend}:8080" \
	-e NGINX_ENVSUBST_TEMPLATE_VARS=API_URL \
	-v "${repo_root}/web/nginx.conf:/etc/nginx/templates/default.conf.template:ro" \
	-v "${html}:/usr/share/nginx/html:ro" \
	"${image}" >/dev/null

base="http://127.0.0.1:${port}"
for _ in $(seq 1 30); do
	curl -sf -o /dev/null "${base}/" && curl -sf -o /dev/null "${base}/healthz" && break
	sleep 1
done

failed=0
expect() {
	local path="$1" want="$2" got
	got="$(curl -s -o /dev/null -w '%{http_code}' "${base}${path}")"
	if [ "${got}" = "${want}" ]; then
		echo "ok   ${path} -> ${got}"
	else
		echo "FAIL ${path} -> ${got}, want ${want}"
		failed=1
	fi
}

# Passes only when the response carries the header exactly once, with that value.
expect_header() {
	local path="$1" header="$2" want="$3" got
	got="$(curl -s -D - -o /dev/null "${base}${path}" | tr -d '\r' |
		awk -v h="${header}:" 'tolower($1) == tolower(h) { sub(/^[^:]*: */, ""); print }')"
	if [ "${got}" = "${want}" ]; then
		echo "ok   ${path} ${header}: ${got}"
	else
		echo "FAIL ${path} ${header}: got [${got//$'\n'/] [}], want [${want}]"
		failed=1
	fi
}

expect / 200
expect /metrics 404
expect '/metrics?format=text' 404
expect /healthz 200

spa_csp="$(sed -n 's/^ *"" *"\(default-src [^"]*\)";$/\1/p' "${repo_root}/web/nginx.conf")"
expect_header / Content-Security-Policy "${spa_csp}"
expect_header / X-Frame-Options SAMEORIGIN
expect_header /metrics X-Content-Type-Options nosniff

# Proxied responses keep the backend's own headers, without the SPA's stacked on.
for path in /docs /api/v1/mcp/servers /agents/acme/bot/.well-known/agent-card.json; do
	expect_header "${path}" Content-Security-Policy "${backend_csp}"
	expect_header "${path}" X-Frame-Options DENY
done

if [ "${failed}" -ne 0 ]; then
	docker logs "${name}" 2>&1 | tail -n 50
	exit 1
fi
