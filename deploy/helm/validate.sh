#!/usr/bin/env bash
# Lints the ai-registry chart, renders it with defaults and each opt-in feature,
# and schema-validates the renders with kubeconform. Run by the Quality and Helm
# publish workflows; needs helm and kubeconform on PATH.
set -euo pipefail

chart="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ai-registry"
issuer=https://idp.example.test/realms/ai-registry

oidc=(
  --set api.oidcEnabled=true
  --set "api.oidcIssuer=${issuer}"
  --set api.oidcClientId=ai-registry-server
  --set api.oidcClientSecret=render-test-secret
)

render() {
  helm template test "$chart" "$@"
}

kubeconform_check() {
  kubeconform -strict -summary -ignore-missing-schemas \
    -schema-location default \
    -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'
}

echo "==> helm lint"
helm lint "$chart"

echo "==> render: defaults (local login only)"
render >/dev/null

echo "==> render: httpRoute"
render --set httpRoute.enabled=true --set httpRoute.gatewayRef.name=test-gateway >/dev/null

echo "==> render: cnpg"
render --set cnpg.enabled=true >/dev/null

echo "==> render: bootstrap sample data"
render --set api.bootstrap.enabled=true --set api.bootstrap.sampleData=true >/dev/null

echo "==> render: oidc broker"
render "${oidc[@]}" | grep -q 'name: test-ai-registry-api-oidc'

echo "==> render: no login method must fail"
if render --set api.localLogin.enabled=false >/dev/null 2>&1; then
  echo "render should have failed when no login method is enabled" >&2
  exit 1
fi

echo "==> kubeconform: defaults"
render | kubeconform_check

echo "==> kubeconform: all opt-ins"
render "${oidc[@]}" \
  --set httpRoute.enabled=true \
  --set httpRoute.gatewayRef.name=test-gateway \
  --set cnpg.enabled=true \
  --set api.bootstrap.enabled=true \
  --set api.bootstrap.sampleData=true \
  --set api.serviceMonitor.enabled=true \
  --set api.podDisruptionBudget.enabled=true \
  --set webapp.podDisruptionBudget.enabled=true \
  | kubeconform_check
