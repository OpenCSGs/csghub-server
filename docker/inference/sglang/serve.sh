#!/bin/bash
if [ "$HF_TASK" == "text-to-video" ] || [ "$HF_TASK" == "image-to-video" ]; then
    # Video generation models (e.g. MiniMax-H3) run on SGLang Diffusion. A repo
    # tagged with both video tasks may deploy as either one.
    bash /etc/csghub/diffusion.sh
elif [ -n "$PD_ROLE" ]; then
    # PD disaggregation mode: delegate to pd-disaggregation_ee.sh
    bash /etc/csghub/pd-disaggregation_ee.sh
elif [ -z "$LWS_WORKER_INDEX" ]; then
    bash /etc/csghub/single-node.sh
else
    bash /etc/csghub/multi-node.sh
fi
