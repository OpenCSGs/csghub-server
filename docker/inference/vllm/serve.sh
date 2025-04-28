#!/bin/bash
if [ -z "$LWS_WORKER_INDEX" ]; then
    bash /etc/csghub/single-node.sh
else
    bash /etc/csghub/multi-node.sh
fi
