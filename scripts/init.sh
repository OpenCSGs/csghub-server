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
echo "Waiting for the database to be ready..."
until telnet postgres 5432 </dev/null 2>&1 | grep -q "Connected"; do
    sleep 1
done
echo "Database is ready!"

# Wait for the Gitea service to be ready
echo "Waiting for Gitea service to be ready..."
until check_gitea; do
    sleep 3
done
echo "Gitea service is ready!"
echo "Running initialization commands..."

# Get the tokens list of $GITEA_USERNAME
tokens=$(curl -s -X GET --url "$STARHUB_SERVER_GITSERVER_HOST/api/v1/users/$GITEA_USERNAME/tokens" --header "Authorization: Basic $AUTH_HEADER")

# Get the first token of tokens
first_token_name=$(echo "$tokens" | jq -r '.[0].name')

# Delete if the access token named `access_token` already exist
if [ -n "$first_token_name" ] && [ "$first_token_name" != "null" ]; then
    echo "Access token already exist, Delete it..."
    curl -s -X DELETE --url "$STARHUB_SERVER_GITSERVER_HOST/api/v1/users/$GITEA_USERNAME/tokens/$first_token_name" --header "Authorization: Basic $AUTH_HEADER"
fi

echo "Creating access token..."
# Create a new access token for $GITEA_USERNAME
TOKEN_RESPONSE=$(curl -s -X POST \
    --url $STARHUB_SERVER_GITSERVER_HOST/api/v1/users/$GITEA_USERNAME/tokens \
    --data-urlencode "name=access_token" \
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

# Add the access token to the environment
echo "export STARHUB_SERVER_GITSERVER_SECRET_KEY=$STARHUB_SERVER_GITSERVER_SECRET_KEY" >> /etc/profile
source /etc/profile

echo "Database setup..."

echo "Migration init"
/starhub-bin/starhub migration init
echo "Migration migrate"
/starhub-bin/starhub migration migrate
echo "Start server..."
/starhub-bin/starhub start server

