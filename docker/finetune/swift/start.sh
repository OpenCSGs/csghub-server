#!/bin/bash

if [ "x${REPO_ID}" != "x" ]; then
    MODEL_NAME=$(echo "$REPO_ID" | cut -d'/' -f2)
fi
if [ ! -f "/workspace/.csghub_init" ]; then
    touch /workspace/.csghub_init
fi

#use csghub model_id_or_path
model_path="/etc/csghub/ms-swift/swift/llm/model/register.py"
argument_path="/etc/csghub/ms-swift/swift/llm/argument/base_args/base_args.py"
sed -i "s|'\([^']*/$MODEL_NAME',\)|'$REPO_ID',|g" $model_path
#use csghub variable 
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" $argument_path
sed -i "s|USE_HF'|USE_CSGHUB_MODEL'|" $model_path
sed -i "s|USE_HF_TRANSFER|USE_CSGHUB_TRANSFER|" $model_path
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" /etc/csghub/ms-swift/swift/hub/hub.py
#change deploy port
sed -i "s|8000|9000|" /etc/csghub/ms-swift/swift/ui/llm_infer/generate.py

template_path="/etc/csghub/ms-swift/swift/llm/utils/template.py"
model_ui_path="/etc/csghub/ms-swift/swift/ui/llm_train/model.py"
#find model type and template type
model_template=`python -W ignore /etc/csghub/get_model_info.py $MODEL_NAME`
echo $model_template
IFS=',' read -ra item_types <<< $model_template
model_type=${item_types[0]}
template_type=${item_types[1]}
lower_transformers=${item_types[2]}
if [ "x${model_type}" != "x" ]; then
    # set default model type for csghub
    grep -q "elem_id='model_type',value=" $model_ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='model_type'|elem_id='model_type',value='${model_type}'|" $model_ui_path
    fi
    # set default template type for csghub
    grep -q "elem_id='template_type',value=" $model_ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='template_type'|elem_id='template_type',value='${template_type}'|" $model_ui_path
    fi
    # set default model id for csghub
    sed -i "s|Qwen/Qwen2.5-7B-Instruct|$REPO_ID|" $model_ui_path
    # set required transformers
    if [ "x${lower_transformers}" = "xyes" ]; then
        pip install transformers==4.33.3
    fi
fi


export GRADIO_ROOT_PATH="${CONTEXT_PATH}/proxy/7860"


ascend_env=/usr/local/Ascend/ascend-toolkit/set_env.sh
if [ -f "$ascend_env" ]; then
    source $ascend_env
    USE_CSGHUB_TRANSFER=1 SWIFT_UI_LANG=en swift web-ui
else
    USE_CSGHUB_TRANSFER=1 SWIFT_UI_LANG=en swift web-ui
fi

