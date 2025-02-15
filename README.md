# hoarder-miniflux-webhook

A webhook service that connects [Miniflux](https://miniflux.app/) (an RSS feed reader) with [Hoarder](https://docs.hoarder.app/) (a "Bookmark Everything" app). This integration allows you to automatically save your Miniflux entries to Hoarder.

## Features

- Save only starred/favorite Miniflux entries to Hoarder
- Optionally save all new Miniflux entries (configurable)
- Works with Docker Compose deployments
- Compatible with reverse proxy setups
- Choose which list you want to save articles to

## Prerequisites

- Docker and Docker Compose
- Running instances of both Hoarder and Miniflux
- Running latest release of Hoarder

## Installation

### 1. Prepare the Environment

1. First, clone this repository into your Hoarder installation directory:

   ```bash
   cd /path/to/your/hoarder/directory
   git clone https://github.com/mathpn/hoarder-miniflux-webhook.git
   ```

2. Create a Docker network to enable communication between Hoarder and Miniflux:

   ```bash
   docker network create service_bridge
   ```

### 2. Set Up Environment Variables

1. Navigate to the cloned repository directory:

   ```bash
   cd hoarder-miniflux-webhook
   ```

2. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

3. Configure the required variables in the `.env` file:

   - `HOARDER_API_TOKEN`: Generate this in Hoarder (Settings → API Keys)
   - `WEBHOOK_SECRET`: This will be generated when enabling webhooks in Miniflux (we'll get this in step 4)
   - `HOARDER_API_URL`: URL of the Hoarder instance (e.g. http://web:3000)
   - `SAVE_NEW_ENTRIES`: Set to `true` to save all new entries (default: `false`)
   - `ADD_TO_LIST`: Set to `true` to save entries to specific list (set list below) (default: `false`)
   - `LIST_ID`: List ID in Hoarder, you will set this on step 5 (default: unset)

### 3. Configure Docker Compose Files

You'll need to modify both your Hoarder and Miniflux Docker Compose configurations to work with the webhook service. Since we're specifying the `service_bridge` network manually, we must also explicitly define the default networks for each service to maintain proper connectivity.

The configurations shown below follow the defaults of each service. Comments indicate what was changed or added. If you have a different configuration, follow the comments to apply the changes to it.

#### Hoarder Configuration (`docker-compose.yml` in your Hoarder directory)

```yaml
services:
  web:
    image: ghcr.io/hoarder-app/hoarder:${HOARDER_VERSION:-release}
    restart: unless-stopped
    volumes:
      - data:/data
    ports:
      - 3000:3000
    networks:
      - hoarder # Default network must be explicitly set
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
      - hoarder # Default network must be explicitly set
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
      - hoarder # Default network must be explicitly set
    env_file:
      - .env
    environment:
      MEILI_NO_ANALYTICS: "true"
    volumes:
      - meilisearch:/meili_data

  # Add webhook service
  hoarder-miniflux-webhook:
    build: ./hoarder-miniflux-webhook
    restart: unless-stopped
    networks:
      - hoarder # Default network must be explicitly set
      - service_bridge # Additional network for inter-service communication

volumes:
  meilisearch:
  data:

networks:
  hoarder: # Default network definition
  service_bridge: # Additional network for inter-service communication
    external: true
```

#### Miniflux Configuration (`docker-compose.yml` in your Miniflux directory)

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
      - miniflux # Default network must be explicitly set
      - service_bridge # Additional network for inter-service communication
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
      - miniflux # Default network must be explicitly set
    volumes:
      - miniflux-db:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "miniflux"]
      interval: 10s
      start_period: 30s

volumes:
  miniflux-db:

networks:
  miniflux: # Default network definition
  service_bridge: # Additional network for inter-service communication
    external: true
```

> **Important:** The default networks (`hoarder` and `miniflux`) must be explicitly defined for each service in their respective Docker Compose files. This is necessary because we're adding the `service_bridge` network manually. Without explicitly setting these networks, services may not be able to communicate with each other properly within their own stack.

### 4. Configure Miniflux Webhook

1. In Miniflux's web interface:

   - Go to Settings → Integrations → Webhook
   - Enable webhook
   - Set the Webhook URL to: `http://hoarder-miniflux-webhook:8080/webhook`
   - Copy the generated webhook secret

2. Paste the webhook secret into your webhook `.env` file:

   ```env
   WEBHOOK_SECRET=your_generated_secret
   ```

### 5. Set `LIST_ID` (optional, required if `ADD_TO_LIST` is true)

You need `curl` installed for this step. Here we use `jq` for code formatting, but it is not strictly required. An alternative is to use `python -m json.tool`.

1. Make a request to list Hoarder lists:

```
curl -H 'Authorization: Bearer <hoarder api key>' -L 'http://<hoarder instance>/api/v1/lists' -H 'Accept: application/json' | jq '.'
```

You'll get something that looks similar this:

```
{
  "lists": [
    {
      "id": "xxxxxxxxxxxxxxxxxxxxxxxx",
      "name": "List One",
      "icon": "📄",
      "parentId": null,
      "type": "manual",
      "query": null
    },
    {
      "id": "xxxxxxxxxxxxxxxxxxxxxxxx",
      "name": "List Two",
      "icon": "📄",
      "parentId": null,
      "type": "manual",
      "query": null
    },
    {
      "id": "xxxxxxxxxxxxxxxxxxxxxxxx",
      "name": "List Three",
      "icon": "📄",
      "parentId": null,
      "type": "manual",
      "query": null
    }
  ]
}
```

2. Choose which list you want Miniflux to add articles to

Copy the string in the `id` field of your list and paste it into your `.env` file in the `LIST_ID` variable.

### 6. Deploy

1. Start/restart the Hoarder services:

   ```bash
   cd /path/to/your/hoarder/directory
   docker compose down
   docker compose build
   docker compose up -d
   ```

2. Start/restart the Miniflux services:

   ```bash
   cd /path/to/your/miniflux/directory
   docker compose down
   docker compose up -d
   ```

## Directory Structure Example

Your directory structure should look something like this:

```
/path/to/your/hoarder/
├── docker-compose.yml
├── .env
└── hoarder-miniflux-webhook/
    ├── .env
    └── [other webhook files]
```

## Important Notes

- If using a reverse proxy, no additional configuration is needed as services communicate through Docker's internal network
- Setting `SAVE_NEW_ENTRIES=true` may result in many entries being saved to Hoarder
- Make sure both `.env` files exist: one for Hoarder and one for the webhook service

## Support

For issues, suggestions, or improvements, please open an issue in the GitHub repository.
