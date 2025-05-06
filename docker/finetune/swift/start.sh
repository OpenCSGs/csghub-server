#!/bin/bash

if [ "x${REPO_ID}" != "x" ]; then
    MODEL_NAME=$(echo "$REPO_ID" | cut -d'/' -f2)
fi
if [ ! -f "/workspace/.csghub_init" ]; then
    touch /workspace/.csghub_init
fi

#use csghub variable
sed -i "s|get_hub(use_hf)|get_hub(True)|g" /etc/csghub/ms-swift/swift/llm/model/utils.py
#change deploy port
sed -i "s|8000|9000|" /etc/csghub/ms-swift/swift/ui/llm_infer/generate.py

template_path="/etc/csghub/ms-swift/swift/llm/utils/template.py"
model_ui_path="/etc/csghub/ms-swift/swift/ui/llm_train/model.py"
infer_ui_path="/etc/csghub/ms-swift/swift/ui/llm_infer/model.py"
export_ui_path="/etc/csghub/ms-swift/swift/ui/llm_export/model.py"
eval_ui_path="/etc/csghub/ms-swift/swift/ui/llm_eval/model.py"
#find model type and template type
model_template=`python -W ignore /etc/csghub/get_model_info.py $MODEL_NAME`
echo $model_template
IFS=',' read -ra item_types <<< $model_template
model_type=${item_types[0]}
template_type=${item_types[1]}
lower_transformers=${item_types[2]}
modify_if_exists() {
    local ui_path=$1
    # set default model type for csghub
    grep -q "elem_id='model_type',value=" $ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='model_type'|elem_id='model_type',value='${model_type}'|" $ui_path
    fi
    # set default template type for csghub
    grep -q "elem_id='template',value=" $ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='template'|elem_id='template',value='${template_type}'|" $ui_path
    fi
    # set default model id for training
    sed -i "s|Qwen/Qwen2.5-7B-Instruct|$REPO_ID|" $ui_path
}
if [ "x${model_type}" != "x" ]; then
    modify_if_exists $model_ui_path
    modify_if_exists $infer_ui_path
    modify_if_exists $export_ui_path
    modify_if_exists $eval_ui_path
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

