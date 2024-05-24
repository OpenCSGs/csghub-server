#!/bin/bash

git clone $HTTPCloneURL code
cp -f code/nginx.conf /etc/nginx/nginx.conf
nginx -g "daemon off;"