#!/bin/bash

if [ "x${REPO_ID}" != "x" ]; then
    MODEL_NAME=$(echo "$REPO_ID" | cut -d'/' -f2)
    grep -qF "${REPO_ID}" /etc/csghub/LLaMA-Factory/src/llamafactory/extras/constants.py
    if [[ $? -eq 0 ]]; then
        grep -qF "\"${MODEL_NAME}\":" /etc/csghub/LLaMA-Factory/src/llamafactory/extras/constants.py   
    fi
    if [[ $? -eq 1 ]]; then
        sed -i "s#CSGHUB_MODEL_NAME#${MODEL_NAME}#" /etc/csghub/extra_models.txt
        sed -i "s#CSGHUB_MODEL_REPO#${REPO_ID}#" /etc/csghub/extra_models.txt
        cat /etc/csghub/extra_models.txt >> /etc/csghub/LLaMA-Factory/src/llamafactory/extras/constants.py  
    fi

    sed -i "s#model_name = gr.Dropdown(choices=available_models, value=None, scale=3)#model_name = gr.Dropdown(choices=available_models,value=\"${MODEL_NAME}\", scale=3)#" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/components/top.py
    sed -i "s#model_path = gr.Textbox(scale=3)#model_path = gr.Textbox(value=\"${REPO_ID}\",scale=3)#" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/components/top.py
    sed -i "s#Hugging Face hub#CSGHub#g" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/locales.py
    sed -i "s#Hugging Face Hub#CSGHub#g" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/locales.py
    sed -i "s#HF Hub#CSGHub#g" /etc/csghub/LLaMA-Factory/src/llamafactory/webui/locales.py
fi
if [ ! -f "/workspace/.csghub_init" ]; then
    if [ ! -d "/workspace/data" ]; then
        cp -rf /etc/csghub/LLaMA-Factory/data /workspace/data
    fi
    if [ ! -d "/workspace/examples" ]; then
        cp -rf /etc/csghub/LLaMA-Factory/examples /workspace/examples
    fi
    if [ ! -d "/workspace/evaluation" ]; then
        cp -rf /etc/csghub/LLaMA-Factory/evaluation /workspace/evaluation
    fi
    #fix revision
    sed -i "s/model_args.model_revision/\"$REVISION\"/g" /etc/csghub/LLaMA-Factory/src/llamafactory/model/loader.py
    touch /workspace/.csghub_init
fi 
#fix upload issue
sed -i "s|and repo_type != constants.REPO_TYPE_MODEL||g" /usr/local/lib/python3.10/dist-packages/huggingface_hub/hf_api.py


export GRADIO_ROOT_PATH="${CONTEXT_PATH}/proxy/7860"
CUDA_VISIBLE_DEVICES=0 llamafactory-cli webui
