#!/bin/bash

if [ "x${REPO_ID}" != "x" ]; then
    MODEL_NAME=$(echo "$REPO_ID" | cut -d'/' -f2)
    grep -qF "${REPO_ID}" /etc/csghub/LLaMA-Factory/src/llamafactory/extras/constants.py
    if [[ $? -eq 1 ]]; then
        sed -i "s#CSGHUB_MODEL_NAME#${MODEL_NAME}#" /etc/csghub/extra_models.txt
        sed -i "s#CSGHUB_MODEL_REPO#${REPO_ID}#" /etc/csghub/extra_models.txt
        cat /etc/csghub/extra_models.txt >> /etc/csghub/LLaMA-Factory/src/llamafactory/extras/constants.py  
    fi

    sed -i "s#model_name = gr.Dropdown(choices=available_models, scale=3)#model_name = gr.Dropdown(choices=available_models,value=\"${MODEL_NAME}\", scale=3)#" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/components/top.py
    sed -i "s#model_path = gr.Textbox(scale=3)#model_path = gr.Textbox(value=\"${REPO_ID}\",scale=3)#" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/components/top.py
fi


llamafactory-cli webui
