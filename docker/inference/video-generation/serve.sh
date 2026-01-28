#!/bin/bash

# Download model
python3 /etc/csghub/entry.py

# Start the server
#if text-to-video
if [ "$HF_TASK" == "text-to-video" ]; then
    /etc/csghub/start_server_t2v.sh
elif [ "$HF_TASK" == "image-to-video" ]; then
    /etc/csghub/start_server_i2v.sh
fi

