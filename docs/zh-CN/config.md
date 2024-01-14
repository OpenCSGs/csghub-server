# 项目配置

| 环境变量名 | 默认值 | 描述 |
| --- | --- | --- |
| STARHUB_SERVER_INSTANCE_ID | none | 一个唯一的实例 ID，用于部署多个实例时做标识 |
| STARHUB_SERVER_ENABLE_SWAGGER | false | 是否开启 Swagger 文档服务 |
| STARHUB_SERVER_API_TOKEN | 0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59 | 用于和前端做身份校验的 API token，长度需要为 128 个字符 |
| STARHUB_SERVER_SERVER_PORT | 8080 | Starhub Sever 启动后监听的端口 |
| STARHUB_SERVER_SERVER_EXTERNAL_HOST | localhost | Starhub Server 启动后的 Host |
| STARHUB_SERVER_SERVER_DOCS_HOST | `http://localhost:6636` | Swagger 启动后的 Host|
| STARHUB_DATABASE_DRIVER | pg | 数据库的类别 |
| STARHUB_DATABASE_DSN | postgresql://postgres:postgres@localhost:5432/STARHUB_SERVER?sslmode=disable | 数据库的 DSN |
| STARHUB_DATABASE_TIMEZONE | Asia/Shanghai | 数据库的时区 |
| STARHUB_SERVER_GITSERVER_URL | http://localhost:3000 | Git server 的地址 |
| STARHUB_SERVER_GITSERVER_TYPE | gitea | Git server 的类别，目前只支持 gitea |
| STARHUB_SERVER_GITSERVER_HOST | http://localhost:3000 | Git server 的 Host |
| STARHUB_SERVER_GITSERVER_SECRET_KEY | 619c849c49e03754454ccd4cda79a209ce0b30b3 | Git server 管理员用户的 access token |
| STARHUB_SERVER_GITSERVER_USERNAME | root | Git server 管理员用户的账号 |
| STARHUB_SERVER_GITSERVER_PASSWORD | password123 | Git server 管理员用户的密码 |
| STARHUB_SERVER_FRONTEND_URL | https://portal-stg.opencsg.com | Starhub 前端项目启动后的 URL |
| STARHUB_SERVER_S3_ACCESS_KEY_ID | none | S3 存储的 Access key ID |
| STARHUB_SERVER_S3_ACCESS_KEY_SECRET | none | S3 存储的 Access key Secret |
| STARHUB_SERVER_S3_REGION | none | S3 存储的 region |
| STARHUB_SERVER_S3_ENDPOINT | none | S3 存储的地址 |
| STARHUB_SERVER_S3_BUCKET | none | S3 存储的 bucket |
| STARHUB_SERVER_SENSITIVE_CHECK_ENABLE | false | 是否开启文本审核(目前只支持阿里云内容审核服务)|
| STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID | none | 阿里云内容审核的 Access key ID |
| STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET | none | 阿里云内容审核的 Access key secret |
| STARHUB_SERVER_SENSITIVE_CHECK_REGION | none | 阿里云内容审核的 region |
| STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT | none | 阿里云内容审核的服务地址 |
