# Environment Variable Configuration Document

## Global Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_SAAS`                   | `false`                         | Whether to enable SaaS mode.                                               |
| `STARHUB_SERVER_INSTANCE_ID`            |                                 | Instance ID used to uniquely identify the current instance.               |
| `STARHUB_SERVER_ENABLE_SWAGGER`         | `false`                         | Whether to enable Swagger documentation.                                  |
| `STARHUB_SERVER_API_TOKEN`              | `0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59`                   | API authentication token used for identity verification.                  |
| `STARHUB_SERVER_ENABLE_HTTPS`           | `false`                         | Whether to enable HTTPS, mainly for reverse proxy usage.                  |
| `STARHUB_SERVER_SERVER_DOCS_HOST`       | `http://localhost:6636`         | Documentation service host address.                                       |

## API Server Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_SERVER_PORT`            | `8080`                          | The port on which the API server listens.                                 |
| `STARHUB_SERVER_PUBLIC_DOMAIN`          | `http://localhost:8080`         | The public domain exposed externally.                                     |
| `STARHUB_SERVER_SSH_DOMAIN`             | `git@localhost:2222`            | SSH domain configuration.                                                 |

## Mirror Service Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_MIRROR_URL`             | `http://localhost:8085`         | The URL address for the mirror service.                                   |
| `STARHUB_SERVER_MIRROR_Token`           |                                 | The authentication token for the mirror service.                          |
| `STARHUB_SERVER_MIRROR_PORT`            | `8085`                          | The port on which the mirror service listens.                              |
| `STARHUB_SERVER_MIRROR_SESSION_SECRET_KEY` | `mirror`                     | The session key for the mirror service.                                   |
| `STARHUB_SERVER_MIRROR_WORKER_NUMBER`   | `5`                             | The number of worker threads for the mirror service.                       |

## Database Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_DATABASE_DRIVER`               | `pg`                            | The database driver type (e.g., PostgreSQL).                               |
| `STARHUB_DATABASE_DSN`                  | `postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable`     | Database connection string.                                                |
| `STARHUB_DATABASE_TIMEZONE`             | `Asia/Shanghai`                 | The timezone used by the database.                                         |

## Redis Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_REDIS_ENDPOINT`         | `localhost:6379`                | Redis server address.                                                     |
| `STARHUB_SERVER_REDIS_MAX_RETRIES`      | `3`                             | Maximum retries for Redis operations.                                      |
| `STARHUB_SERVER_REDIS_MIN_IDLE_CONNECTIONS` | `0`                          | Minimum idle Redis connections.                                            |
| `STARHUB_SERVER_REDIS_USER`             |                                 | Redis username.                                                           |
| `STARHUB_SERVER_REDIS_PASSWORD`         |                                 | Redis password.                                                           |
| `STARHUB_SERVER_REDIS_USE_SENTINEL`     | `false`                         | Whether to use Redis Sentinel mode.                                        |
| `STARHUB_SERVER_REDIS_SENTINEL_MASTER`  |                                 | The name of the Sentinel master node.                                     |
| `STARHUB_SERVER_REDIS_SENTINEL_ENDPOINT`|                                 | The address of the Sentinel node.                                          |

## Git Server Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_GITSERVER_URL`          | `http://localhost:3000`         | Git server URL address.                                                    |
| `STARHUB_SERVER_GITSERVER_TYPE`         | `gitea`                         | The type of the Git server (e.g., Gitea).                                  |
| `STARHUB_SERVER_GITSERVER_HOST`         | `http://localhost:3000`         | Git server host address.                                                   |
| `STARHUB_SERVER_GITSERVER_SECRET_KEY`   | `619c849c49e03754454ccd4cda79a209ce0b30b3`                   | Authentication key for the Git server.                                    |
| `STARHUB_SERVER_GITSERVER_USERNAME`     | `root`                          | Git server username.                                                       |
| `STARHUB_SERVER_GITSERVER_PASSWORD`     | `password123`                   | Git server password.                                                       |
| `STARHUB_SERVER_GITSERVER_TIMEOUT_SEC`  | `5`                             | Git server request timeout (in seconds).                                   |

## Gitaly Configuration

| Environment Variable                    | Default Value                   | Type     | Description                                                                |
|-----------------------------------------|---------------------------------|----------|----------------------------------------------------------------------------|
| `STARHUB_SERVER_GITALY_SERVER_SOCKET`   | `tcp://localhost:9999`          | `string` | The Gitaly server address.                                                 |
| `STARHUB_SERVER_GITALY_STORGE`          | `default`                       | `string` | The Gitaly storage type.                                                   |
| `STARHUB_SERVER_GITALY_TOKEN`           | `abc123secret`                  | `string` | The Gitaly token.                                                          |
| `STARHUB_SERVER_GITALY_JWT_SECRET`      | `signing-key`                   | `string` | The Gitaly JWT signing key.                                                |

## Mirror Server Configuration

| Environment Variable                    | Default Value                   | Type     | Description                                                                |
|-----------------------------------------|---------------------------------|----------|----------------------------------------------------------------------------|
| `STARHUB_SERVER_MIRRORSERVER_ENABLE`    | `false`                         | `bool`   | Whether to enable the mirror server.                                       |
| `STARHUB_SERVER_MIRRORSERVER_URL`       | `http://localhost:3001`         | `string` | The URL address for the mirror server.                                     |
| `STARHUB_SERVER_MIRRORSERVER_TYPE`      | `gitea`                         | `string` | The type of the mirror server.                                             |
| `STARHUB_SERVER_MIRRORSERVER_HOST`      | `http://localhost:3001`         | `string` | The host address for the mirror server.                                    |
| `STARHUB_SERVER_MIRRORSERVER_SECRET_KEY`| `619c849c49e03754454ccd4cda79a209ce0b30b3` | `string` | The authentication key for the mirror server.                              |
| `STARHUB_SERVER_MIRRORSERVER_USERNAME`  | `root`                          | `string` | The username for the mirror server.                                        |
| `STARHUB_SERVER_MIRRORSERVER_PASSWORD`  | `password123`                   | `string` | The password for the mirror server.                                        |

## Frontend Configuration

| Environment Variable                    | Default Value                   | Type     | Description                                                                |
|-----------------------------------------|---------------------------------|----------|----------------------------------------------------------------------------|
| `STARHUB_SERVER_FRONTEND_URL`           | `https://opencsg.com`           | `string` | The frontend service URL address.                                          |

## S3 Configuration

| Environment Variable                    | Default Value                   | Description                                                                 |
|-----------------------------------------|---------------------------------|---------------------------------------------------------------------------|
| `STARHUB_SERVER_S3_SSL`                 | `false`                         | Whether to enable SSL for S3.                                              |
| `STARHUB_SERVER_S3_ACCESS_KEY_ID`       |                                 | S3 access key ID.                                                          |
| `STARHUB_SERVER_S3_ACCESS_KEY_SECRET`   |                                 | S3 access key secret.                                                      |
| `STARHUB_SERVER_S3_REGION`              |                                 | S3 region configuration.                                                   |
| `STARHUB_SERVER_S3_ENDPOINT`            | `localhost:9000`                | The S3 server address.                                                     |
| `STARHUB_SERVER_S3_INTERNAL_ENDPOINT`   |                                 | The internal S3 server address.                                            |
| `STARHUB_SERVER_S3_BUCKET`              | `opencsg-test`                  | The S3 bucket name.                                                        |
| `STARHUB_SERVER_S3_ENABLE_SSL`          | `false`                         | Whether to enable SSL.                                                     |

## Sensitive Word Detection Configuration

| Environment Variable                              | Default Value                 | Type   | Description                                                                |
|---------------------------------------------------|-------------------------------|--------|----------------------------------------------------------------------------|
| `STARHUB_SERVER_SENSITIVE_CHECK_ENABLE`           | `false`                       | `bool` | Whether to enable sensitive word detection.                                |
| `STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID`    |                               | `string` | OSS access key ID.                                                        |
| `STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET`|                               | `string` | OSS access key secret.                                                    |
| `STARHUB_SERVER_SENSITIVE_CHECK_REGION`           |                               | `string` | OSS region name.                                                          |
| `STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT`         | `oss-cn-beijing.aliyuncs.com` | `string` | OSS endpoint.                                                            |
| `STARHUB_SERVER_S3_ENABLE_SSH`                    | `true`                        | `bool`  | Whether to enable SSH.                                                    |

## JWT Configuration

| Environment Variable                    | Default Value                   | Type     | Description                                                                |
|-----------------------------------------|---------------------------------|----------|----------------------------------------------------------------------------|
| `STARHUB_SERVER_JWT_SIGNING_KEY`         | `secret-key`                    | `string` | JWT signing key.                                                          |
| `STARHUB_SERVER_JWT_EXPIRES_IN`          | `24h`                           | `string` | JWT expiration time.                                                       |

## Space Configuration

| Environment Variable                                | Default Value                                       | Type      | Description                             |
|-----------------------------------------------------|----------------------------------------------------|-----------|-----------------------------------------|
| `STARHUB_SERVER_SPACE_BUILDER_ENDPOINT`             | `http://localhost:8081`                            | `string`  | Builder service address.               |
| `STARHUB_SERVER_SPACE_RUNNER_ENDPOINT`              | `http://localhost:8082`                            | `string`  | Runner service address.                |
| `STARHUB_SERVER_SPACE_RUNNER_SERVER_PORT`           | `8082`                                            | `int`     | Runner service port.                   |
| `STARHUB_SERVER_INTERNAL_ROOT_DOMAIN`               | `internal.example.com`                             | `string`  | Internal root domain (for internal access). |
| `STARHUB_SERVER_PUBLIC_ROOT_DOMAIN`                 | `public.example.com`                               | `string`  | Public root domain.                    |
| `STARHUB_SERVER_DOCKER_REG_BASE`                    | `registry.cn-beijing.aliyuncs.com/opencsg_public/` | `string`  | Base address for Docker image repository. |
| `STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET`           | `opencsg-pull-secret`                              | `string`  | Secret name for pulling images.        |
| `STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT`           | `8083`                                            | `int`     | Reverse proxy service port.            |
| `STARHUB_SERVER_SPACE_SESSION_SECRET_KEY`           | `secret`                                          | `string`  | Session encryption key.                |
| `STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES`    | `30`                                              | `int`     | Deployment timeout in minutes.         |
| `STARHUB_SERVER_GPU_MODEL_LABEL`                    | `aliyun.accelerator/nvidia_name`                   | `string`  | GPU model label.                       |
| `STARHUB_SERVER_READNESS_DELAY_SECONDS`             | `120`                                             | `int`     | Readiness check delay time in seconds. |
| `STARHUB_SERVER_READNESS_PERIOD_SECONDS`            | `10`                                              | `int`     | Readiness check interval time in seconds. |
| `STARHUB_SERVER_READNESS_FAILURE_THRESHOLD`         | `3`                                               | `int`     | Readiness check failure threshold.     |
                                    |

## Model Configuration

| Environment Variable                                | Default Value                                        | Type      | Description                           |
|-----------------------------------------------------|-----------------------------------------------------|-----------|---------------------------------------|
| `STARHUB_SERVER_MODEL_DEPLOY_TIMEOUT_IN_MINUTES`    | `60`                                                 | `int`     | Model deployment timeout in minutes. |
| `STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT`            | `https://hub.opencsg.com`                            | `string`  | Model download address.              |
| `STARHUB_SERVER_MODEL_DOCKER_REG_BASE`              | `opencsg-registry.cn-beijing.cr.aliyuncs.com/public/` | `string`  | Base address for model Docker image repository. |
| `STARHUB_SERVER_MODEL_NIM_DOCKER_SECRET_NAME`       | `ngc-secret`                                         | `string`  | NGC Docker image secret name.        |
| `STARHUB_SERVER_MODEL_NIM_NGC_SECRET_NAME`          | `nvidia-nim-secrets`                                 | `string`  | NGC Secret name.                     |


## Event Configuration

| Environment Variable                    | Default Value | Type  | Description                        |
|-----------------------------------------|---------------|-------|------------------------------------|
| `STARHUB_SERVER_SYNC_IN_MINUTES`       | `1`           | `int` | Interval time for event synchronization (in minutes). |


## Casdoor Configuration

| Environment Variable                      | Default Value                     | Type    | Description                       |
|-------------------------------------------|-----------------------------------|---------|-----------------------------------|
| `STARHUB_SERVER_CASDOOR_CLIENT_ID`        | `client_id`                       | `string`| Casdoor client ID.                |
| `STARHUB_SERVER_CASDOOR_CLIENT_SECRET`    | `client_secret`                   | `string`| Casdoor client secret.            |
| `STARHUB_SERVER_CASDOOR_ENDPOINT`         | `http://localhost:80`             | `string`| Casdoor server endpoint address.  |
| `STARHUB_SERVER_CASDOOR_CERTIFICATE`      | `/etc/casdoor/certificate.pem`    | `string`| Casdoor certificate path.         |
| `STARHUB_SERVER_CASDOOR_ORGANIZATION_NAME`| `opencsg`                         | `string`| Casdoor organization name.        |
| `STARHUB_SERVER_CASDOOR_APPLICATION_NAME` | `opencsg`                         | `string`| Casdoor application name.         |


## Nats Configuration

| Environment Variable                          | Default Value                                                   | Type    | Description                          |
|-----------------------------------------------|-----------------------------------------------------------------|---------|--------------------------------------|
| `OPENCSG_ACCOUNTING_NATS_URL`                 | `nats://account:g98dc5FA8v4J7ck90w@natsmaster:4222`             | `string`| NATS service URL.                   |
| `OPENCSG_ACCOUNTING_MSG_FETCH_TIMEOUTINSEC`   | `5`                                                             | `int`   | Message fetch timeout (seconds).    |
| `OPENCSG_ACCOUNTING_METER_EVENT_SUBJECT`      | `accounting.metering.>`                                         | `string`| Message subject for metering events. |
| `STARHUB_SERVER_METER_DURATION_SEND_SUBJECT`  | `accounting.metering.duration`                                  | `string`| Message subject for sending duration data. |
| `STARHUB_SERVER_METER_TOKEN_SEND_SUBJECT`     | `accounting.metering.token`                                     | `string`| Message subject for sending token data. |
| `STARHUB_SERVER_METER_QUOTA_SEND_SUBJECT`     | `accounting.metering.quota`                                     | `string`| Message subject for sending quota data. |


## User Configuration

| Environment Variable                                      | Default Value                             | Type    | Description                          |
|-----------------------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_USER_SERVER_HOST`                                | `http://localhost`                        | `string`| User service host address.          |
| `OPENCSG_USER_SERVER_PORT`                                | `8088`                                    | `int`   | User service port number.           |
| `OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL`         | `http://localhost:3000/server/callback`   | `string`| Redirect URL after successful login. |

## Multi-source Synchronization Configuration

| Environment Variable                                      | Default Value                             | Type    | Description                          |
|-----------------------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_SAAS_API_DOMAIN`                                 | `https://hub.opencsg.com`                 | `string`| SaaS API domain.                    |
| `OPENCSG_SAAS_SYNC_DOMAIN`                                | `https://sync.opencsg.com`                | `string`| SaaS synchronization domain.        |
| `STARHUB_SERVER_MULTI_SYNC_ENABLED`                       | `true`                                    | `bool`  | Whether multi-sync functionality is enabled. |
                          |

## Telemetry Configuration

| Environment Variable                        | Default Value                             | Type    | Description                          |
|---------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `STARHUB_SERVER_TELEMETRY_ENABLE`           | `true`                                    | `bool`  | Whether telemetry is enabled.        |
| `STARHUB_SERVER_TELEMETRY_URL`              | `http://hub.opencsg.com/api/v1/telemetry` | `string`| Telemetry report URL.               |

## Auto Cleanup Configuration

| Environment Variable                        | Default Value                             | Type    | Description                          |
|---------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_AUTO_CLEANUP_INSTANCE_ENABLE`      | `false`                                   | `bool`  | Whether automatic instance cleanup is enabled. |

## Dataset Configuration

| Environment Variable                        | Default Value                             | Type      | Description                          |
|---------------------------------------------|-------------------------------------------|-----------|--------------------------------------|
| `OPENCSG_PROMPT_MAX_JSONL_FILESIZE_BYTES`   | `1048576` (1MB)                           | `int64`   | Maximum dataset file size (bytes).  |

## Data Flow Configuration

| Environment Variable                        | Default Value                             | Type    | Description                          |
|---------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_DATAFLOW_SERVER_HOST`              | `http://127.0.0.1`                       | `string`| Data flow service host address.     |
| `OPENCSG_DATAFLOW_SERVER_PORT`              | `8000`                                    | `int`    | Data flow service port number.      |

## Moderation Configuration

| Environment Variable                        | Default Value                             | Type    | Description                          |
|---------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_MODERATION_SERVER_HOST`            | `http://localhost`                        | `string`| Moderation service host address.    |
| `OPENCSG_MODERATION_SERVER_PORT`            | `8089`                                    | `int`    | Moderation service port number.     |
| `OPENCSG_MODERATION_SERVER_ENCODED_SENSITIVE_WORDS` | `5Lmg6L+R5bmzLHhpamlucGluZw==`   | `string`| Encoded sensitive words list.       |

## Workflow Configuration

| Environment Variable                        | Default Value                             | Type    | Description                          |
|---------------------------------------------|-------------------------------------------|---------|--------------------------------------|
| `OPENCSG_WORKFLOW_SERVER_ENDPOINT`          | `localhost:7233`                          | `string`| Workflow service endpoint.           |
