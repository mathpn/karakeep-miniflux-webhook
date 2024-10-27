# hoarder-miniflux-webhook

Simple service to integrate Hoarder with Miniflux using a webhook.

## Setup

The setup presented below uses Docker Compose for both Hoarder and Miniflux.

### Create a shared bridge Docker network

This is required if each service is running on separate Docker networks and if they don't have public IP addresses (i.e. not exposed to the internet). This is the default behavior if there are separate docker compose files. The network will allow communication between the services.

```bash
docker network create service_bridge
```

### Modify both docker compose files to include the new network

Since we're specifying a network manually, we also need to create a default network for all services. Use the examples below as **examples** and change what is needed.

#### Hoarder

Add the new networks to each service as well as in the top-level `networks` key. This example is derived from the [hoarder repo](https://github.com/hoarder-app/hoarder/blob/main/docker/docker-compose.yml) with additional networking configuration. Only the `web` service needs access to the `service_bridge` network.

**Change the `build` key of `miniflux-integration` to the path of this repository.**

```yaml
version: "3.8"
services:
  web:
    image: ghcr.io/hoarder-app/hoarder:${HOARDER_VERSION:-release}
    restart: unless-stopped
    volumes:
      - data:/data
    ports:
      - 3000:3000
    networks:
      - hoarder
      - service_bridge
    env_file:
      - .env
    environment:
      MEILI_ADDR: http://meilisearch:7700
      BROWSER_WEB_URL: http://chrome:9222
      # OPENAI_API_KEY: ...
      DATA_DIR: /data
  chrome:
    image: gcr.io/zenika-hub/alpine-chrome:123
    restart: unless-stopped
    networks:
      - hoarder
    command:
      - --no-sandbox
      - --disable-gpu
      - --disable-dev-shm-usage
      - --remote-debugging-address=0.0.0.0
      - --remote-debugging-port=9222
      - --hide-scrollbars
  meilisearch:
    image: getmeili/meilisearch:v1.6
    restart: unless-stopped
    networks:
      - hoarder
    env_file:
      - .env
    environment:
      MEILI_NO_ANALYTICS: "true"
    volumes:
      - meilisearch:/meili_data

  hoarder-miniflux-webhook:
    build: ../hoarder-miniflux-webhook
    restart: unless-stopped
    networks:
      - hoarder
      - service_bridge

volumes:
  meilisearch:
  data:

networks:
  hoarder:
  service_bridge:
    external: true
```

#### Miniflux

Add the new networks to each service as well as in the top-level `networks` key. Only the `miniflux` container needs to access the `service_bridge` network. The example configuration below was adapted from [the Miniflux documentation](https://miniflux.app/docs/docker.html).

```yaml
services:
  miniflux:
    image: miniflux/miniflux:latest
    ports:
      - "80:8080"
    depends_on:
      db:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "/usr/bin/miniflux", "-healthcheck", "auto"]
    env_file: .env
    networks:
      - service_bridge
      - miniflux
    environment:
      - DATABASE_URL=postgres://miniflux:secret@db/miniflux?sslmode=disable
      - RUN_MIGRATIONS=1
      - CREATE_ADMIN=1
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=test123
  db:
    image: postgres:15
    environment:
      - POSTGRES_USER=miniflux
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=miniflux
    networks:
      - miniflux
    volumes:
      - miniflux-db:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "miniflux"]
      interval: 10s
      start_period: 30s

volumes:
  miniflux-db:

networks:
  service_bridge:
    external: true
  miniflux:
```

### Fill environment variables

Copy the `.env.example` file to `.env` and make the necessary changes. If you follow the same setup provided above, you only need to fill `HOARDER_API_TOKEN` and `WEBHOOK_SECRET`. Generate the Hoarder API token directly in the Hoarder web interface (Settings → API Keys).

As stated in the [documentation](https://miniflux.app/docs/webhooks.html), the webhook secret is generated when you enable the webhook integration (Settings → Integrations → Webhook → Enable webhook). Copy the secret to the `.env` file. The Webhook URL should be in the format `http://<SERVICE_NAME>:<PORT>/webhook`. Using the example configuration provided here: `http://hoarder-miniflux-webhook:8080/webhook`.

### Restart the Hoarder services

In the directory of the Hoarder docker compose file:

```bash
docker compose down
docker compose build
docker compose up -d
```
