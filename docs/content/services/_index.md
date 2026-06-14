---
title: "Services"
weight: 3
---

localaz runs every emulated Azure service in a single process, each on its own
port. The pages in this section cover the individual data-plane and
control-plane services. This page documents the cross-cutting **management**
endpoints.

## Management endpoints

Alongside the emulated services, localaz serves one **plain HTTP** listener (no
TLS) on port `8000` by default. This is the management endpoint: it is not an
emulated Azure service but a home for cross-cutting operational endpoints. Change its address with `-management-addr` / `LOCALAZ_MANAGEMENT_ADDR`
(see [Configuration](/configuration)).

| Endpoint | Method | Description |
| -------- | ------ | ----------- |
| `/health` | `GET` | `200 OK` once every service is bound and listening, `503` until then. |
| `/certs/pubkey` | `GET` | The PEM-encoded certificate the TLS services are served with. |
| `/certs/privkey` | `GET` | The PEM-encoded private key for that certificate. |

The private key is served unauthenticated. The certificate served here is the
one the TLS services use — the auto-generated self-signed pair, or the
`-tls-cert` / `-tls-key` pair you supplied.

### Readiness

The `/health` endpoint returns `503` while the emulator is still starting and
`200 OK` once every listener is bound and serving. Wait for it before issuing
requests:

```bash
until curl -fsS http://127.0.0.1:8000/health; do sleep 0.2; done
```

### Fetching the certificate

Fetch the certificate clients need to trust the TLS services without reading it
off disk:

```bash
curl -fsS http://127.0.0.1:8000/certs/pubkey -o localaz.crt
export SSL_CERT_FILE="$PWD/localaz.crt"
export REQUESTS_CA_BUNDLE="$SSL_CERT_FILE"
```

## Container health check

The Docker image ships a `HEALTHCHECK` that runs a built-in probe of the
`/health` endpoint. The container is reported healthy only once every service
is listening.

### Usage with Docker

```bash
docker run --wait linksociety/localaz:latest
```

### Usage with Docker Compose

```yaml
services:
  localaz:
    image: linksociety/localaz:latest
    ports:
      - "10000:10000"
      - "8000:8000"

  app:
    image: my-app
    depends_on:
      localaz:
        condition: service_healthy
```
