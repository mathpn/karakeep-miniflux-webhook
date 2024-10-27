# hoarder-miniflux-webhook

A webhook service that connects [Miniflux](https://miniflux.app/) (an RSS feed reader) with [Hoarder](https://docs.hoarder.app/) (a "Bookmark Everything" app). This integration allows you to automatically save your Miniflux entries to Hoarder.

## Features

- Save only starred/favorite Miniflux entries to Hoarder
- Optionally save all new Miniflux entries (configurable)
- Works with Docker Compose deployments
- Compatible with reverse proxy setups

## Prerequisites

- Docker and Docker Compose
- Running instances of both Hoarder and Miniflux
- Running latest release of Hoarder

## Installation

### 1. Create a Bridge Network

First, create a Docker network to enable communication between Hoarder and Miniflux:

```bash
docker network create service_bridge
```

### 2. Configure Docker Compose Files

Since we're specifying a network manually, we also need to create a default network for all services. Use the examples below as **examples** and change what is needed.

#### Hoarder Configuration

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

#### Miniflux Configuration

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

### 3. Set Up Environment Variables

1. Copy `.env.example` to `.env`
2. Configure the required variables:
   - `HOARDER_API_TOKEN`: Generate this in Hoarder (Settings → API Keys)
   - `WEBHOOK_SECRET`: Generated when enabling webhooks in Miniflux, as described [here](https://miniflux.app/docs/webhooks.html)
   - `SAVE_NEW_ENTRIES`: Set to `true` to save all new entries (default: `false`)

### 4. Configure Miniflux Webhook

In Miniflux:

1. Go to Settings → Integrations → Webhook
2. Enable webhook
3. Set the Webhook URL to: `http://hoarder-miniflux-webhook:8080/webhook`
4. Copy the generated webhook secret to your `.env` file

### 5. Deploy

Restart the Hoarder services:

```bash
docker compose down
docker compose build
docker compose up -d
```

Restart the Miniflux services:

```bash
docker compose down
docker compose up -d
```

## Important Notes

- If using a reverse proxy, no additional configuration is needed as services communicate through Docker's internal network
- Setting `SAVE_NEW_ENTRIES=true` may result in a large number of entries being saved to Hoarder

## Support

For issues, suggestions, or improvements, please open an issue in the GitHub repository.
