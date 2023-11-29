# Starhub API

This is an API project that provides services to `portal`.

## Build

```shell
cd cmd/starhub-server/ && go build
```

## Usage

### Database migration

```shell
# init migration tables
./starhub-server migration init

# create sql migration files
./starhub-server migration create_go <filename>

# create go migration files
./starhub-server migration create_sql <filename>

# execute migration
./starhub-server migration migrate

# rollback the last migration group
./starhub-server migration rollback
```

### Configurations

| Environment Variable Name | Default Value | Detail |
| --- | --- | --- |
| STARHUB_SERVER_INSTANCE_ID | none | Primary instance ID |
| STARHUB_SERVER_ENABLE_SWAGGER | false | Whether to open the Swagger API documentation page |
| STARHUB_SERVER_SERVER_PORT | 8080 | The port of starhub-server server |
| STARHUB_SERVER_SERVER_EXTERNAL_HOST | localhost | The external host of starhub-server server |
| STARHUB_SERVER_SERVER_DOCS_HOST | `http://localhost:6636` | The host of documentation page |
| STARHUB_DATABASE_DRIVER | pg | Database driver name |
| STARHUB_DATABASE_DSN | postgresql://postgres:postgres@localhost:5432/STARHUB_SERVER?sslmode=disable | Database DSN |
| STARHUB_DATABASE_TIMEZONE | Asia/Shanghai | The timezone used by the database |
| STARHUB_SERVER_REDIS_ENDPOINT | localhost:6379 | Redis endpoint |
| STARHUB_SERVER_REDIS_MAX_RETRIES | 3 | Max retry count of Redis |
| STARHUB_SERVER_REDIS_MIN_IDLE_CONNECTIONS | 0 | Minimum number of free connections held in the connection pool |
| STARHUB_SERVER_REDIS_USER | none | The username of Redis server |
| STARHUB_SERVER_REDIS_PASSWORD | none | The password of Redis server |
| STARHUB_SERVER_REDIS_USE_SENTINEL | false | Used to enable or disable the Sentinel function |
| STARHUB_SERVER_REDIS_SENTINEL_MASTER | none | The name of master Redis node |
| STARHUB_SERVER_REDIS_SENTINEL_ENDPOINT | none | The endpoint of master Redis node |
| STARHUB_SERVER_GITSERVER_TYPE | gitea | The type of Git server [`gitea` or `local`] |
| STARHUB_SERVER_GITSERVER_HOST | none | The host of Git server |
| STARHUB_SERVER_GITSERVER_SECRET_KEY | none | The secret key of Git server |

### Server

```shell
# start server with binary
./starhub-server start server

# start all services (Gitea, PG, Redis) with docker compose
docker compose up -d
```
