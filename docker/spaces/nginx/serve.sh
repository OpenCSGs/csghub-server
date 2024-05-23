#!/bin/bash

REPO="https://$ACCESS_TOKEN@opencsg.com/spaces/$REPO_ID.git"
git clone $REPO code
cp -f code/nginx.conf /etc/nginx/nginx.conf
nginx -g "daemon off;"