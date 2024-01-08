#!/bin/bash

export STARHUB_DATABASE_DSN="postgresql://postgres:postgres@localhost:5433/starhub_server?sslmode=disable"
export STARHUB_DATABASE_TIMEZONE="Asia/Shanghai"
export STARHUB_SERVER_REDIS_ENDPOINT="localhost:6379"
export STARHUB_SERVER_GITSERVER_HOST="http://localhost:3001"
# export STARHUB_SERVER_GITSERVER_SECRET_KEY="46389362f3c3e2aadc6083b94c0f226337a98b6d"
# export STARHUB_SERVER_GITSERVER_HOST="http://localhost:3000"
export STARHUB_SERVER_GITSERVER_SECRET_KEY="cbcfa2497b51a6f75adcff8421a6b8da808fe505"
export STARHUB_SERVER_GITSERVER_USERNAME="root"
export STARHUB_SERVER_GITSERVER_PASSWORD="password123"
# export STARHUB_SERVER_GITSERVER_USERNAME="leida"
# export STARHUB_SERVER_GITSERVER_PASSWORD="12345678"
export GITEA_SERVICE_DEFAULT_ALLOW_CREATE_ORGANIZATION=true
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="starhub_server"
export GITEA_USERNAME="root"
export GITEA_PASSWORD="password123"
#export GIN_MODE="release"

#allow Bun to log sql queries
export DB_DEBUG=2

#maker sure you have defined these 3 environment variables
# export STARHUB_SERVER_ALIYUN_ACCESS_KEY_ID="[YOUR_ACCESS_KEY_ID]"
# export STARHUB_SERVER_ALIYUN_ACCESS_KEY_SECRET="[YOUR_ACCESS_KEY_SECRET]"
# export STARHUB_SERVER_ALIYUN_REGION="[YOUR_ALIYUN_REGION]"

go build -v -o ./bin/starhub-server ./cmd/starhub-server

./bin/starhub-server migration init
#uncomment this command if db schema changed
# ./bin/starhub-server migration create_sql add_col_count_to_repository_tags
./bin/starhub-server migration migrate 

./bin/starhub-server start server -l wrong_level -f json 
