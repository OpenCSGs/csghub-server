# 环境变量配置文档

## 全局配置

| 环境变量                        | 默认值                         | 描述                                                                                     |
|--------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_SAAS`          | `false`                       | 是否启用 SaaS 模式。                                                                   |
| `STARHUB_SERVER_INSTANCE_ID`   |                               | 实例 ID，用于唯一标识当前实例。                                                        |
| `STARHUB_SERVER_ENABLE_SWAGGER`| `false`                       | 是否启用 Swagger 文档。                                                                |
| `STARHUB_SERVER_API_TOKEN`     | `0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59`                 | API 鉴权令牌，用于身份验证。                                                           |
| `STARHUB_SERVER_ENABLE_HTTPS`  | `false`                       | 是否启用 HTTPS，主要用于反向代理。                                                     |
| `STARHUB_SERVER_SERVER_DOCS_HOST` | `http://localhost:6636`     | 文档服务的主机地址。                                                                   |

## API 服务器配置

| 环境变量                          | 默认值                         | 描述                                                                                     |
|----------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_SERVER_PORT`     | `8080`                        | API 服务器监听的端口。                                                                  |
| `STARHUB_SERVER_PUBLIC_DOMAIN`   | `http://localhost:8080`       | 对外暴露的公共域名。                                                                   |
| `STARHUB_SERVER_SSH_DOMAIN`      | `git@localhost:2222`          | SSH 域名配置。                                                                          |

## 镜像服务配置

| 环境变量                          | 默认值                         | 描述                                                                                     |
|----------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_MIRROR_URL`      | `http://localhost:8085`       | 镜像服务的 URL 地址。                                                                  |
| `STARHUB_SERVER_MIRROR_Token`    |                               | 镜像服务的鉴权令牌。                                                                   |
| `STARHUB_SERVER_MIRROR_PORT`     | `8085`                        | 镜像服务监听的端口。                                                                   |
| `STARHUB_SERVER_MIRROR_SESSION_SECRET_KEY` | `mirror`          | 镜像服务会话的密钥。                                                                   |
| `STARHUB_SERVER_MIRROR_WORKER_NUMBER` | `5`                       | 镜像服务的工作线程数。                                                                 |

## 数据库配置

| 环境变量                          | 默认值                         | 描述                                                                                     |
|----------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_DATABASE_DRIVER`        | `pg`                          | 数据库驱动类型（如 PostgreSQL）。                                                       |
| `STARHUB_DATABASE_DSN`           | `postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable`   | 数据库连接字符串。                                                                      |
| `STARHUB_DATABASE_TIMEZONE`      | `Asia/Shanghai`               | 数据库使用的时区。                                                                      |

## Redis 配置

| 环境变量                              | 默认值                         | 描述                                                                                     |
|--------------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_REDIS_ENDPOINT`      | `localhost:6379`              | Redis 服务器的地址。                                                                    |
| `STARHUB_SERVER_REDIS_MAX_RETRIES`   | `3`                           | Redis 操作的最大重试次数。                                                              |
| `STARHUB_SERVER_REDIS_MIN_IDLE_CONNECTIONS` | `0`                        | Redis 最小空闲连接数。                                                                  |
| `STARHUB_SERVER_REDIS_USER`          |                               | Redis 用户名。                                                                          |
| `STARHUB_SERVER_REDIS_PASSWORD`      |                               | Redis 密码。                                                                            |
| `STARHUB_SERVER_REDIS_USE_SENTINEL`  | `false`                       | 是否使用 Redis Sentinel 模式。                                                          |
| `STARHUB_SERVER_REDIS_SENTINEL_MASTER` |                              | Sentinel 主节点名称。                                                                  |
| `STARHUB_SERVER_REDIS_SENTINEL_ENDPOINT` |                           | Sentinel 节点的地址。                                                                  |

## Git 服务器配置

| 环境变量                           | 默认值                         | 描述                                                                                     |
|-----------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_GITSERVER_URL`    | `http://localhost:3000`       | Git 服务器的 URL 地址。                                                                 |
| `STARHUB_SERVER_GITSERVER_TYPE`   | `gitea`                       | Git 服务器类型（如 Gitea）。                                                            |
| `STARHUB_SERVER_GITSERVER_HOST`   | `http://localhost:3000`       | Git 服务器主机地址。                                                                    |
| `STARHUB_SERVER_GITSERVER_SECRET_KEY` | `619c849c49e03754454ccd4cda79a209ce0b30b3`              | Git 服务器鉴权密钥。                                                                    |
| `STARHUB_SERVER_GITSERVER_USERNAME` | `root`                      | Git 服务器用户名。                                                                      |
| `STARHUB_SERVER_GITSERVER_PASSWORD` | `password123`               | Git 服务器密码。                                                                        |
| `STARHUB_SERVER_GITSERVER_TIMEOUT_SEC` | `5`                       | Git 服务器请求超时时间（秒）。                                                          |

## Gitaly 配置
| 环境变量                                | 默认值                              | 类型      | 说明                          |
|-----------------------------------------|-------------------------------------|-----------|-------------------------------|
| `STARHUB_SERVER_GITALY_SERVER_SOCKET`   | `tcp://localhost:9999`              | `string`  | Gitaly 服务器地址。           |
| `STARHUB_SERVER_GITALY_STORGE`          | `default`                           | `string`  | Gitaly 存储类型。             |
| `STARHUB_SERVER_GITALY_TOKEN`           | `abc123secret`                      | `string`  | Gitaly Token。                |
| `STARHUB_SERVER_GITALY_JWT_SECRET`      | `signing-key`                       | `string`  | Gitaly JWT 签名密钥。         |

## 镜像服务配置
| 环境变量                                | 默认值                              | 类型      | 说明                          |
|-----------------------------------------|-------------------------------------|-----------|-------------------------------|
| `STARHUB_SERVER_MIRRORSERVER_ENABLE`    | `false`                             | `bool`    | 是否启用镜像服务器。          |
| `STARHUB_SERVER_MIRRORSERVER_URL`       | `http://localhost:3001`             | `string`  | 镜像服务器 URL。              |
| `STARHUB_SERVER_MIRRORSERVER_TYPE`      | `gitea`                             | `string`  | 镜像服务器类型。              |
| `STARHUB_SERVER_MIRRORSERVER_HOST`      | `http://localhost:3001`             | `string`  | 镜像服务器主机地址。          |
| `STARHUB_SERVER_MIRRORSERVER_SECRET_KEY`| `619c849c49e03754454ccd4cda79a209ce0b30b3` | `string`  | 镜像服务器密钥。              |
| `STARHUB_SERVER_MIRRORSERVER_USERNAME`  | `root`                              | `string`  | 镜像服务器用户名。            |
| `STARHUB_SERVER_MIRRORSERVER_PASSWORD`  | `password123`                       | `string`  | 镜像服务器密码。              |

## 前端配置
| 环境变量                                | 默认值                              | 类型      | 说明                          |
|-----------------------------------------|-------------------------------------|-----------|-------------------------------|
| `STARHUB_SERVER_FRONTEND_URL`           | `https://opencsg.com`               | `string`  | 前端服务的 URL 地址。         |


## S3 配置

| 环境变量                            | 默认值                         | 描述                                                                                     |
|------------------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `STARHUB_SERVER_S3_SSL`            | `false`                       | 是否启用 S3 的 SSL。                                                                    |
| `STARHUB_SERVER_S3_ACCESS_KEY_ID`  |                               | S3 的访问密钥 ID。                                                                      |
| `STARHUB_SERVER_S3_ACCESS_KEY_SECRET` |                            | S3 的访问密钥 Secret。                                                                  |
| `STARHUB_SERVER_S3_REGION`         |                               | S3 的区域配置。                                                                          |
| `STARHUB_SERVER_S3_ENDPOINT`       | `localhost:9000`              | S3 的服务端地址。                                                                          |
| `STARHUB_SERVER_S3_INTERNAL_ENDPOINT`         |                    | S3 内部服务端地址。                                                                       |
| `STARHUB_SERVER_S3_BUCKET`         | `opencsg-test`                | S3 的存储桶名称。                                                                       |
| `STARHUB_SERVER_S3_ENABLE_SSL`         | false                     | 是否启用 SSL。                                                                       |


## 敏感词检测配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `STARHUB_SERVER_SENSITIVE_CHECK_ENABLE` | `false`                             | `bool`| 是否启用敏感信息检查功能。      |
| `STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID` | -                             | `string` | OSS 的访问密钥 ID。           |
| `STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET` | -                          | `string` | OSS 的访问密钥 Secret。       |
| `STARHUB_SERVER_SENSITIVE_CHECK_REGION` | -                                   | `string` | OSS 的区域名称。              |
| `STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT` | `oss-cn-beijing.aliyuncs.com`     | `string` | OSS 的访问终端。              |
| `STARHUB_SERVER_S3_ENABLE_SSH`         | `true`                              | `bool` | 是否启用 SSL。                |

## JWT 配置
| 环境变量                      | 默认值           | 类型      | 说明                          |
|-------------------------------|------------------|-----------|-------------------------------|
| `STARHUB_JWT_SIGNING_KEY`     | `signing-key`    | `string`  | JWT 签名密钥。                |
| `STARHUB_JWT_VALIDATE_HOUR`   | `24`             | `int`     | JWT 的有效时间（小时）。       |


## 应用配置
| 环境变量                                | 默认值                                       | 类型      | 说明                          |
|-----------------------------------------|----------------------------------------------|-----------|-------------------------------|
| `STARHUB_SERVER_SPACE_BUILDER_ENDPOINT` | `http://localhost:8081`                      | `string`  | 构建器服务地址。              |
| `STARHUB_SERVER_SPACE_RUNNER_ENDPOINT`  | `http://localhost:8082`                      | `string`  | 运行器服务地址。              |
| `STARHUB_SERVER_SPACE_RUNNER_SERVER_PORT`| `8082`                                      | `int`     | 运行器服务端口。              |
| `STARHUB_SERVER_INTERNAL_ROOT_DOMAIN`   | `internal.example.com`                       | `string`  | 内部根域名（仅内部访问）。    |
| `STARHUB_SERVER_PUBLIC_ROOT_DOMAIN`     | `public.example.com`                         | `string`  | 公共根域名。                  |
| `STARHUB_SERVER_DOCKER_REG_BASE`        | `registry.cn-beijing.aliyuncs.com/opencsg_public/` | `string`  | Docker 镜像仓库基地址。       |
| `STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET`| `opencsg-pull-secret`                        | `string`  | 拉取镜像的秘密名称。          |
| `STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT`| `8083`                                      | `int`     | 反向代理服务端口。            |
| `STARHUB_SERVER_SPACE_SESSION_SECRET_KEY`| `secret`                                    | `string`  | 会话加密密钥。                |
| `STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES` | `30`                                 | `int`     | 部署超时时间（分钟）。        |
| `STARHUB_SERVER_GPU_MODEL_LABEL`        | `aliyun.accelerator/nvidia_name`             | `string`  | GPU 模型标签。                |
| `STARHUB_SERVER_READNESS_DELAY_SECONDS` | `120`                                       | `int`     | 健康检查延迟时间（秒）。      |
| `STARHUB_SERVER_READNESS_PERIOD_SECONDS`| `10`                                        | `int`     | 健康检查间隔时间（秒）。      |
| `STARHUB_SERVER_READNESS_FAILURE_THRESHOLD` | `3`                                    | `int`     | 健康检查失败阈值。            |

## 模型配置
| 环境变量                                | 默认值                                       | 类型      | 说明                          |
|-----------------------------------------|----------------------------------------------|-----------|-------------------------------|
| `STARHUB_SERVER_MODEL_DEPLOY_TIMEOUT_IN_MINUTES` | `60`                                 | `int`     | 模型部署超时时间（分钟）。    |
| `STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT`| `https://hub.opencsg.com`                   | `string`  | 模型下载地址。                |
| `STARHUB_SERVER_MODEL_DOCKER_REG_BASE`  | `opencsg-registry.cn-beijing.cr.aliyuncs.com/public/` | `string`  | 模型 Docker 镜像仓库基地址。  |
| `STARHUB_SERVER_MODEL_NIM_DOCKER_SECRET_NAME` | `ngc-secret`                          | `string`  | NGC Docker 镜像秘密名称。     |
| `STARHUB_SERVER_MODEL_NIM_NGC_SECRET_NAME` | `nvidia-nim-secrets`                  | `string`  | NGC Secret 名称。             |


## 事件配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `STARHUB_SERVER_SYNC_IN_MINUTES`       | `1`                                 | `int`  | 同步事件的间隔时间（分钟）。   |

## Casdoor 配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `STARHUB_SERVER_CASDOOR_CLIENT_ID`      | `client_id`                         | `string` | Casdoor 客户端 ID。           |
| `STARHUB_SERVER_CASDOOR_CLIENT_SECRET`  | `client_secret`                     | `string` | Casdoor 客户端密钥。          |
| `STARHUB_SERVER_CASDOOR_ENDPOINT`       | `http://localhost:80`               | `string` | Casdoor 服务端点地址。        |
| `STARHUB_SERVER_CASDOOR_CERTIFICATE`    | `/etc/casdoor/certificate.pem`      | `string` | Casdoor 证书路径。            |
| `STARHUB_SERVER_CASDOOR_ORGANIZATION_NAME` | `opencsg`                        | `string` | Casdoor 组织名称。            |
| `STARHUB_SERVER_CASDOOR_APPLICATION_NAME` | `opencsg`                         | `string` | Casdoor 应用名称。            |

## Nats 配置
| 环境变量                                  | 默认值                                       | 类型      | 说明                          |
|-------------------------------------------|----------------------------------------------|-----------|-------------------------------|
| `OPENCSG_ACCOUNTING_NATS_URL`             | `nats://account:g98dc5FA8v4J7ck90w@natsmaster:4222` | `string`  | NATS 服务 URL。               |
| `OPENCSG_ACCOUNTING_MSG_FETCH_TIMEOUTINSEC` | `5`                                     | `int`     | 消息获取超时（秒）。          |
| `OPENCSG_ACCOUNTING_METER_EVENT_SUBJECT`  | `accounting.metering.>`                     | `string`  | 用于计费事件的消息主题。      |
| `STARHUB_SERVER_METER_DURATION_SEND_SUBJECT` | `accounting.metering.duration`          | `string`  | 用于发送时长数据的消息主题。  |
| `STARHUB_SERVER_METER_TOKEN_SEND_SUBJECT` | `accounting.metering.token`                | `string`  | 用于发送 token 数据的消息主题。 |
| `STARHUB_SERVER_METER_QUOTA_SEND_SUBJECT` | `accounting.metering.quota`                | `string`  | 用于发送配额数据的消息主题。  |

## 用户配置
| 环境变量                                      | 默认值                              | 类型      | 说明                          |
|-----------------------------------------------|-------------------------------------|-----------|-------------------------------|
| `OPENCSG_USER_SERVER_HOST`                    | `http://localhost`                  | `string`  | 用户服务的主机地址。          |
| `OPENCSG_USER_SERVER_PORT`                    | `8088`                              | `int`     | 用户服务的端口号。            |
| `OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL` | `http://localhost:3000/server/callback` | `string`  | 登录成功后的重定向 URL。      |

## 多源同步配置
| 环境变量                                | 默认值                                       | 类型      | 说明                          |
|-----------------------------------------|----------------------------------------------|-----------|-------------------------------|
| `OPENCSG_SAAS_API_DOMAIN`               | `https://hub.opencsg.com`                   | `string`  | SaaS API 域名。               |
| `OPENCSG_SAAS_SYNC_DOMAIN`              | `https://sync.opencsg.com`                  | `string`  | SaaS 同步域名。               |
| `STARHUB_SERVER_MULTI_SYNC_ENABLED`     | `true`                                      | `bool`    | 是否启用多同步功能。          |

## 遥测配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `STARHUB_SERVER_TELEMETRY_ENABLE`       | `true`                              | `bool` | 是否启用遥测功能。            |
| `STARHUB_SERVER_TELEMETRY_URL`          | `http://hub.opencsg.com/api/v1/telemetry` | `string` | 遥测报告 URL 地址。           |

## 自动清理配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `OPENCSG_AUTO_CLEANUP_INSTANCE_ENABLE`  | `false`                             | `bool` | 是否启用自动清理实例功能。    |

## 数据集配置
| 环境变量                                | 默认值                              | 类型      | 说明                          |
|-----------------------------------------|-------------------------------------|-----------|-------------------------------|
| `OPENCSG_PROMPT_MAX_JSONL_FILESIZE_BYTES` | `1048576` (1MB)                    | `int64`   | 数据集文件最大大小（字节）。  |

## 数据流配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `OPENCSG_DATAFLOW_SERVER_HOST`          | `http://127.0.0.1`                 | `string` | 数据流服务的主机地址。        |
| `OPENCSG_DATAFLOW_SERVER_PORT`          | `8000`                              | `int`    | 数据流服务的端口号。          |

## 审核配置
| 环境变量                                | 默认值                              | 类型  | 说明                          |
|-----------------------------------------|-------------------------------------|-------|-------------------------------|
| `OPENCSG_MODERATION_SERVER_HOST`        | `http://localhost`                  | `string` | 审核服务的主机地址。          |
| `OPENCSG_MODERATION_SERVER_PORT`        | `8089`                              | `int`    | 审核服务的端口号。            |
| `OPENCSG_MODERATION_SERVER_ENCODED_SENSITIVE_WORDS` | `5Lmg6L+R5bmzLHhpamlucGluZw==` | `string` | 编码的敏感词列表。            |

## 工作流配置
| 环境变量                                | 默认值                              | 类型      | 说明                          |
|-----------------------------------------|-------------------------------------|-----------|-------------------------------|
| `OPENCSG_WORKFLOW_SERVER_ENDPOINT`      | `localhost:7233`                    | `string`  | 工作流服务端点。              |

