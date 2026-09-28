#!/usr/bin/env bash
# Runs web/nginx.conf in the web image's nginx base and checks the paths that
# must never reach the backend through the public entry point.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
port="${NGINX_TEST_PORT:-18080}"
name="ai-registry-nginx-test-$$"
image="$(awk '$1 == "FROM" && $NF == "runner" { print $2 }' "${repo_root}/web/Dockerfile")"

html="$(mktemp -d)"
chmod 755 "${html}"
echo '<!doctype html><title>spa</title>' >"${html}/index.html"
chmod 644 "${html}/index.html"

cleanup() {
	docker rm -f "${name}" >/dev/null 2>&1 || true
	rm -rf "${html}"
}
trap cleanup EXIT

# API_URL points at a closed port: proxied paths answer 502, which is enough to
# tell "proxied" apart from "refused by nginx".
docker run -d --name "${name}" -p "127.0.0.1:${port}:8080" \
	-e API_URL=http://127.0.0.1:9 \
	-e NGINX_ENVSUBST_TEMPLATE_VARS=API_URL \
	-v "${repo_root}/web/nginx.conf:/etc/nginx/templates/default.conf.template:ro" \
	-v "${html}:/usr/share/nginx/html:ro" \
	"${image}" >/dev/null

base="http://127.0.0.1:${port}"
for _ in $(seq 1 30); do
	curl -sf -o /dev/null "${base}/" && break
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

expect / 200
expect /metrics 404
expect '/metrics?format=text' 404
expect /healthz 502

if ! curl -s -D - -o /dev/null "${base}/metrics" | grep -qi '^x-content-type-options: nosniff'; then
	echo "FAIL /metrics 404 lost the server-level security headers"
	failed=1
fi

if [ "${failed}" -ne 0 ]; then
	docker logs "${name}" 2>&1 | tail -n 50
	exit 1
fi
