#!/bin/bash

export STARHUB_DATABASE_DSN=postgresql://postgres:postgres@localhost:5433/starhub_server?sslmode=disable
export STARHUB_DATABASE_TIMEZONE=Asia/Shanghai
export STARHUB_SERVER_GITSERVER_HOST=http://localhost:3001
export STARHUB_SERVER_GITSERVER_URL=http://localhost:3001
export STARHUB_SERVER_GITSERVER_USERNAME=root
export STARHUB_SERVER_GITSERVER_PASSWORD=password123
export STARHUB_SERVER_GITSERVER_WEBHOOK_URL=http://localhost:8080/api/v1/callback/git
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=starhub_server
export GITEA_USERNAME=root
export GITEA_PASSWORD=password123
export GIN_MODE=release
#  export STARHUB_SERVER_API_TOKEN=$STARHUB_SERVER_API_TOKEN
export STARHUB_SERVER_S3_ACCESS_KEY_ID=$STARHUB_SERVER_S3_ACCESS_KEY_ID
export STARHUB_SERVER_S3_ACCESS_KEY_SECRET=$STARHUB_SERVER_S3_ACCESS_KEY_SECRET
export STARHUB_SERVER_S3_REGION=$STARHUB_SERVER_S3_REGION
#  export STARHUB_SERVER_S3_BUCKET: $STARHUB_SERVER_S3_BUCKET
export OPENCSG_ACCOUNTING_NATS_URL=nats://natsadmin:cE90aPsV7nws83xubzP3ce3F9xg@127.0.0.1:4222
export OPENCSG_ACCOUNTING_FEE_EVENT_SUBJECT="accounting.fee.>"
export OPENCSG_ACCOUNTING_NOTIFY_NOBALANCE_SUBJECT="accounting.notify.nobalance"

#export GIN_MODE="release"

#allow Bun to log sql queries
export DB_DEBUG=1

# cd ..
echo $(pwd)

~/go/bin/swag init -d cmd/csghub-server/cmd/start,api/router,api/handler,builder/store/database,common/types,accounting/router,accounting/handler,accounting/types -g server.go

go build -v -o ./bin/csghub-server ./cmd/csghub-server

# make sure build succeeded
if [ $? -ne 0 ]; then
        echo "go build failed"
        exit 1
fi

./bin/csghub-server migration init
#uncomment this command if db schema changed
#./bin/csghub-server migration create_sql create_table_spaces
./bin/csghub-server migration migrate

./bin/csghub-server start server -l Info -f json --swagger true
#./bin/csghub-server trigger fix-org-data -l DEBUG -f json