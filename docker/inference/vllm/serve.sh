#!/bin/bash
if [ -n "$PD_ROLE" ]; then
    # PD disaggregation mode: delegate to pd-disaggregation.sh
    bash /etc/csghub/pd-disaggregation.sh
elif [ -z "$LWS_WORKER_INDEX" ]; then
    bash /etc/csghub/single-node.sh
else
    bash /etc/csghub/multi-node.sh
fi
