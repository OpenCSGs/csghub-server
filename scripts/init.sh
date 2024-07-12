#!/bin/bash

# The base64 encoded usename:password
AUTH_HEADER=$(echo -n "$GITEA_USERNAME:$GITEA_PASSWORD" | base64)
# Function to check if the Gitea service is ready
check_gitea() {
    # Check the availability of the Gitea service
    # Replace the following command with the appropriate check for your service
    # For example, using curl to check if the Gitea API responds:
    curl -s -X GET --url $STARHUB_SERVER_GITSERVER_HOST/api/v1/version --header "Authorization: Basic $AUTH_HEADER" | grep "version"
}

# Wait for the database to be ready
# echo "Waiting for the database to be ready..."
# until telnet postgres 5432 </dev/null 2>&1 | grep -q "Connected"; do
#     sleep 1
# done
# echo "Database is ready!"

# Wait for the Gitea service to be ready
echo "Waiting for Gitea service to be ready..."
until check_gitea; do
    sleep 3
done
echo "Gitea service is ready!"
echo "Running initialization commands..."


# Delete if the access token named `webhook_access_token` already exist
echo "Access token already exist, Delete it..."
curl -s -X DELETE --url "$STARHUB_SERVER_GITSERVER_HOST/api/v1/users/$GITEA_USERNAME/tokens/webhook_access_token" --header "Authorization: Basic $AUTH_HEADER"

echo "Creating access token..."
# Create a new access token for $GITEA_USERNAME
TOKEN_RESPONSE=$(curl -s -X POST \
    --url $STARHUB_SERVER_GITSERVER_HOST/api/v1/users/$GITEA_USERNAME/tokens \
    --data-urlencode "name=webhook_access_token" \
    --data-urlencode "scopes=read:user,write:user,write:admin,read:admin" \
    --header "accept: application/json" \
    --header "Content-Type: application/x-www-form-urlencoded" \
    --header "Authorization: Basic $AUTH_HEADER")

# Extract access token from the response
STARHUB_SERVER_GITSERVER_SECRET_KEY=$(echo "$TOKEN_RESPONSE" | jq -r '.sha1')

# Get the system hook list
webhooks=$(curl -s -X GET --url "$STARHUB_SERVER_GITSERVER_HOST/api/v1/admin/hooks" --header "Authorization: Basic $AUTH_HEADER")

# Get the first hook type
first_hook_type=$(echo "$webhooks" | jq -r '.[0].type')

if [ -n "$first_hook_type" ] && [ "$first_hook_type" != "null" ]; then
    echo "System hook exists"
else
    # Create a webhook to send push events
    curl -X POST \
        -H "Content-Type: application/json" \
        -d '{
        "type": "gitea",
        "authorization_header": "Bearer '"$STARHUB_SERVER_API_TOKEN"'",
        "config": {
            "is_system_webhook": "true",
            "url": "'"$STARHUB_SERVER_GITSERVER_WEBHOOK_URL"'",
            "content_type": "json",
            "insecure_ssl": "true"
        },
        "events": ["push"],
        "active": true
        }' \
        "$STARHUB_SERVER_GITSERVER_HOST/api/v1/admin/hooks?access_token=$STARHUB_SERVER_GITSERVER_SECRET_KEY"
fi

# Create cron job
cron=""
read_and_set_cron() {
    env_variable=$1
    default_value=$2
    
    cron=${!env_variable}

    if [[ -z $cron ]]; then
        cron=$default_value
    fi
}

current_cron_jobs=$(crontab -l 2>/dev/null)

if echo "$current_cron_jobs" | grep -qF "starhub logscan gitea"; then
    echo "Gitea log scan job already exists"
else
    echo "Creating cron job for gitea logscan..."
    read_and_set_cron "STARHUB_SERVER_CRON_LOGSCAN" "0 23 * * *"
    (crontab -l ;echo "$cron STARHUB_DATABASE_DSN=$STARHUB_DATABASE_DSN /starhub-bin/starhub logscan gitea --path /starhub-bin/logs/gitea.log >> /starhub-bin/cron.log 2>&1") | crontab -
fi

if echo "$current_cron_jobs" | grep -qF "calc-recom-score"; then
    echo "Calculate score job already exists"
else
    echo "Creating cron job for repository recommendation score calculation..."
    read_and_set_cron "STARHUB_SERVER_CRON_CALC_RECOM_SCORE" "0 1 * * *"
    (crontab -l ;echo "$cron STARHUB_DATABASE_DSN=$STARHUB_DATABASE_DSN STARHUB_SERVER_GITSERVER_HOST=$STARHUB_SERVER_GITSERVER_HOST STARHUB_SERVER_GITSERVER_USERNAME=$STARHUB_SERVER_GITSERVER_USERNAME STARHUB_SERVER_GITSERVER_PASSWORD=$STARHUB_SERVER_GITSERVER_PASSWORD /starhub-bin/starhub cron calc-recom-score >> /starhub-bin/cron-calc-recom-score.log 2>&1") | crontab -
fi

if echo "$current_cron_jobs" | grep -qF "create-push-mirror"; then
    echo "Create push mirror job already exists"
else
    echo "Creating cron job for push mirror creation..."
    read_and_set_cron "STARHUB_SERVER_CRON_PUSH_MIRROR" "*/10 * * * *"
    (crontab -l ;echo "$cron STARHUB_DATABASE_DSN=$STARHUB_DATABASE_DSN STARHUB_SERVER_GITSERVER_HOST=$STARHUB_SERVER_GITSERVER_HOST STARHUB_SERVER_GITSERVER_USERNAME=$STARHUB_SERVER_GITSERVER_USERNAME STARHUB_SERVER_GITSERVER_PASSWORD=$STARHUB_SERVER_GITSERVER_PASSWORD STARHUB_SERVER_MIRRORSERVER_HOST=$STARHUB_SERVER_MIRRORSERVER_HOST STARHUB_SERVER_MIRRORSERVER_USERNAME=$STARHUB_SERVER_MIRRORSERVER_USERNAME STARHUB_SERVER_MIRRORSERVER_PASSWORD=$STARHUB_SERVER_MIRRORSERVER_PASSWORD /starhub-bin/starhub cron create-push-mirror >> /starhub-bin/create-push-mirror.log 2>&1") | crontab -
fi

if echo "$current_cron_jobs" | grep -qF "check-mirror-progress"; then
    echo "Check mirror progress job already exists"
else
    echo "Creating cron job for update mirror status and progress..."
    read_and_set_cron "STARHUB_SERVER_CRON_PUSH_MIRROR" "*/5 * * * *"
    (crontab -l ;echo "$cron STARHUB_DATABASE_DSN=$STARHUB_DATABASE_DSN STARHUB_SERVER_GITSERVER_HOST=$STARHUB_SERVER_GITSERVER_HOST STARHUB_SERVER_GITSERVER_USERNAME=$STARHUB_SERVER_GITSERVER_USERNAME STARHUB_SERVER_GITSERVER_PASSWORD=$STARHUB_SERVER_GITSERVER_PASSWORD STARHUB_SERVER_MIRRORSERVER_HOST=$STARHUB_SERVER_MIRRORSERVER_HOST STARHUB_SERVER_MIRRORSERVER_USERNAME=$STARHUB_SERVER_MIRRORSERVER_USERNAME STARHUB_SERVER_MIRRORSERVER_PASSWORD=$STARHUB_SERVER_MIRRORSERVER_PASSWORD STARHUB_SERVER_REDIS_ENDPOINT=$STARHUB_SERVER_REDIS_ENDPOINT STARHUB_SERVER_REDIS_USER=$STARHUB_SERVER_REDIS_USER STARHUB_SERVER_REDIS_PASSWORD=$STARHUB_SERVER_REDIS_PASSWORD /starhub-bin/starhub mirror check-mirror-progress >> /starhub-bin/check-mirror-progress.log 2>&1") | crontab -
fi

if [ "$STARHUB_SERVER_SAAS" == "false" ]; then
    if echo "$current_cron_jobs" | grep -qF "sync-as-client"; then
        echo "Sync as client job already exists"
    else
        echo "Creating cron job for sync saas sync verions..."
        read_and_set_cron "STARHUB_SERVER_CRON_SYNC_AS_CLIENT" "0 * * * *"
        (crontab -l ;echo "$cron STARHUB_DATABASE_DSN=$STARHUB_DATABASE_DSN STARHUB_SERVER_GITSERVER_HOST=$STARHUB_SERVER_GITSERVER_HOST STARHUB_SERVER_GITSERVER_USERNAME=$STARHUB_SERVER_GITSERVER_USERNAME STARHUB_SERVER_GITSERVER_PASSWORD=$STARHUB_SERVER_GITSERVER_PASSWORD STARHUB_SERVER_REDIS_ENDPOINT=$STARHUB_SERVER_REDIS_ENDPOINT STARHUB_SERVER_REDIS_USER=$STARHUB_SERVER_REDIS_USER STARHUB_SERVER_REDIS_PASSWORD=$STARHUB_SERVER_REDIS_PASSWORD /starhub-bin/starhub cron sync-as-client >> /starhub-bin/cron-sync-as-client.log 2>&1") | crontab -
    fi
else
    echo "Saas does not need sync-as-client cron job"
fi
# Reload cron server
service cron restart
echo "Done."

echo "Database setup..."

echo "Migration init"
/starhub-bin/starhub migration init
echo "Migration migrate"
/starhub-bin/starhub migration migrate
echo "Start server..."
/starhub-bin/starhub start server

