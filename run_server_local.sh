#!/bin/bash

export STARHUB_DATABASE_DSN="postgresql://postgres:postgres@localhost:5433/starhub_server?sslmode=disable"
export STARHUB_DATABASE_TIMEZONE="Asia/Shanghai"
export STARHUB_SERVER_REDIS_ENDPOINT="localhost:6379"
export STARHUB_SERVER_GITSERVER_HOST="http://localhost:3001"
export STARHUB_SERVER_GITSERVER_USERNAME="root"
export STARHUB_SERVER_GITSERVER_PASSWORD="password123"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="starhub_server"
export GITEA_USERNAME="root"
export GITEA_PASSWORD="password123"
#export GIN_MODE="release"

go build -o ./starhub-server ./cmd/starhub-server

./starhub-server start server
