#!/bin/bash

export STARHUB_DATABASE_DSN="postgresql://postgres:postgres@localhost:5433/starhub_server?sslmode=disable"
export STARHUB_DATABASE_TIMEZONE="Asia/Shanghai"
export STARHUB_SERVER_REDIS_ENDPOINT="localhost:6379"
export STARHUB_SERVER_GITSERVER_HOST="http://localhost:3001"
export STARHUB_SERVER_GITSERVER_SECRET_KEY="46389362f3c3e2aadc6083b94c0f226337a98b6d"
export STARHUB_SERVER_GITSERVER_USERNAME="root"
export STARHUB_SERVER_GITSERVER_PASSWORD="password123"
export GITEA_SERVICE_DEFAULT_ALLOW_CREATE_ORGANIZATION=true
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="starhub_server"
export GITEA_USERNAME="root"
export GITEA_PASSWORD="password123"
#export GIN_MODE="release"

go build -v -o ./bin/starhub-server ./cmd/starhub-server

./bin/starhub-server migration init
./bin/starhub-server migration migrate

./bin/starhub-server start server
