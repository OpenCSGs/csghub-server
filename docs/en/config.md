# Project Configuration

| Environment Variable | Default Value | Description |
| --- | --- | --- |
| STARHUB_SERVER_INSTANCE_ID | none | A unique instance ID used to identify multiple instances during deployment |
| STARHUB_SERVER_ENABLE_SWAGGER | false | Whether to enable Swagger documentation service |
| STARHUB_SERVER_API_TOKEN | 0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59 | API token for identity verification with the frontend, it needs to be 128 characters long |
| STARHUB_SERVER_SERVER_PORT | 8080 | Port on which CSGhub Server listens after startup |
| STARHUB_SERVER_SERVER_EXTERNAL_HOST | localhost | Host after CSGhub Server startup |
| STARHUB_SERVER_SERVER_DOCS_HOST | `http://localhost:6636` | Host after Swagger startup |
| STARHUB_DATABASE_DRIVER | pg | Database type |
| STARHUB_DATABASE_DSN | postgresql://postgres:postgres@localhost:5432/STARHUB_SERVER?sslmode=disable | Database DSN |
| STARHUB_DATABASE_TIMEZONE | Asia/Shanghai | Database timezone |
| STARHUB_SERVER_GITSERVER_URL | http://localhost:3000 | Git server address |
| STARHUB_SERVER_GITSERVER_TYPE | gitea | Git server type, currently only supports gitea |
| STARHUB_SERVER_GITSERVER_HOST | http://localhost:3000 | Git server host |
| STARHUB_SERVER_GITSERVER_SECRET_KEY | 619c849c49e03754454ccd4cda79a209ce0b30b3 | Access token for Git server administrator user |
| STARHUB_SERVER_GITSERVER_USERNAME | root | Account of the Git server administrator user |
| STARHUB_SERVER_GITSERVER_PASSWORD | password123 | Password of the Git server administrator user |
| STARHUB_SERVER_FRONTEND_URL | https://portal-stg.opencsg.com | URL after CSGhub frontend project startup |
| STARHUB_SERVER_S3_ACCESS_KEY_ID | none | S3 storage Access key ID |
| STARHUB_SERVER_S3_ACCESS_KEY_SECRET | none | S3 storage Access key Secret |
| STARHUB_SERVER_S3_REGION | none | S3 storage region |
| STARHUB_SERVER_S3_ENDPOINT | none | S3 storage address |
| STARHUB_SERVER_S3_BUCKET | none | S3 storage bucket |
| STARHUB_SERVER_SENSITIVE_CHECK_ENABLE | false | Whether to enable text review (currently only supports Alibaba Cloud content review service) |
| STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID | none | Alibaba Cloud content review Access key ID |
| STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET | none | Alibaba Cloud content review Access key secret |
| STARHUB_SERVER_SENSITIVE_CHECK_REGION | none | Alibaba Cloud content review region |
| STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT | none | Alibaba Cloud content review service address |
