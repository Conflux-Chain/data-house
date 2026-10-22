# data-house

Conflux blockchain data infrastructure. The sync workers and monitor run as Docker Compose services.

## Configure

Create the private environment file locally. It is ignored by Git:

```sh
cp .env.example .env
# Edit .env and set DH_STORE_MYSQL_PASSWORD and the alert webhook.
```

The per-service database names, adapter URLs, alert tags, and EVM starting blocks are also configurable in `.env`. The values in `.env.example` match the original launch scripts.

## Start containers

Start all services in the background:

```sh
docker compose up --build -d
```

Start one service with its corresponding configuration:

```sh
./1029.sh
./1030.sh
./1run.sh
./71run.sh
./monitor.sh
```

The scripts read `.env` and run the matching Compose service. They keep the logs attached. To start one service in the background, use `docker compose up --build -d net1029`.

The monitor API is available at `http://localhost:${DH_MONITOR_PORT:-8080}/dh/progress`.

## Inspect and stop

```sh
docker compose ps
docker compose logs -f net1029
docker compose down
```
