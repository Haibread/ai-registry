# Load / smoke tests

k6 scripts for post-deploy verification.

## Smoke test

Fast (~30 s) check that the public read-only surface is healthy. Use this as
a gate after a production deploy, or in CI against a staging environment.

```sh
# Against a running docker-compose dev stack
k6 run test/load/smoke.js

# Against a deployed environment
k6 run test/load/smoke.js -e BASE_URL=https://ai-registry.example.com
```

Exits non-zero if any of these thresholds fail:

- p95 latency > 500 ms
- >1 % of checks failed
- >1 % of requests errored (connection refused, DNS, TLS)

## Installing k6

```sh
# macOS
brew install k6

# Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

## Future scripts

- `soak.js` — multi-hour soak at nominal load; catches leaks / FD exhaustion.
- `spike.js` — sudden 10x burst; validates rate limiting and HPA behaviour.
- `admin-write.js` — authenticated admin write path; requires a service-account
  token via `-e ADMIN_TOKEN=…`.
