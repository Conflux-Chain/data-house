# data-house

Conflux blockchain data infrastructure. The sync workers and monitor run as Docker Compose services.

## Start

```sh
cp .env.example .env
# Edit .env with the database credentials and alert settings.
docker compose up --build -d
```

This starts `net1`, `net71`, `net1029`, `net1030`, and `monitor`. Start one service with `docker compose up --build net1029`, or use the existing scripts (`./1029.sh`, `./1030.sh`, `./1run.sh`, `./71run.sh`, and `./monitor.sh`).

The monitor API is available at `http://localhost:${DH_MONITOR_PORT:-8080}/dh/progress`.
Node adapter URLs, database names, and EVM starting blocks have per-service environment variables in `docker-compose.yaml`.

## Stop and inspect

```sh
docker compose down
docker compose logs -f net1029
```
