#!/bin/bash

if [ "x${REPO_ID}" != "x" ]; then
    MODEL_NAME=$(echo "$REPO_ID" | cut -d'/' -f2)
fi
if [ ! -f "/workspace/.csghub_init" ]; then
    touch /workspace/.csghub_init
fi

#use csghub model_id_or_path
model_path="/etc/csghub/ms-swift/swift/llm/utils/model.py"
argument_path="/etc/csghub/ms-swift/swift/llm/utils/argument.py"
sed -i "s|'\([^']*/$MODEL_NAME',\)|'$REPO_ID',|g" $model_path
#use csghub variable 
sed -i "s|USE_HF|USE_CSGHUB_DATASET|" /etc/csghub/ms-swift/swift/llm/utils/dataset.py
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" $argument_path
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" $model_path
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" /etc/csghub/ms-swift/swift/trainers/push_to_ms.py


template_path="/etc/csghub/ms-swift/swift/llm/utils/template.py"
model_ui_path="/etc/csghub/ms-swift/swift/ui/llm_train/model.py"
hf_model_id=`grep -E "hf_model_id='.*\/$MODEL_NAME'" model.py | awk -F"'" '{print $2}'`

#find model type and template type
types=`awk -v model_id="${hf_model_id}" '
BEGIN {RS=")"; FS="\n"}
/@register_model/ {
    for(i=1; i<=NF; i++) {
        if($i ~ /ModelType\./) {
            split($i, arrModelType, ".");
            modelType = arrModelType[2];
        }
        if($i ~ /TemplateType\./) {
            split($i, arrTemplateType, ".");
            templateType = arrTemplateType[2];
        }
        if($i ~ /hf_model_id/ && index($0, "hf_model_id='\''" model_id "'\''")) {
            print modelType templateType;
            break;
        }
    }
}
' $model_path`
IFS=',' read -ra item_types <<< $types
model_type=${item_types[0]}
template_type=${item_types[1]}
if [ "x${model_type}" != "x" ]; then
    model_type_name=`awk -v search="$model_type =" -F"'" '$0 ~ search {print $2}' $model_path`
    template_type_name=`awk -v search=" $template_type =" -F"'" '$0 ~ search {print $2}' $template_path`
    # set default model type for csghub
    grep -q "elem_id='model_type',value=" $model_ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='model_type'|elem_id='model_type',value='${model_type_name}'|" $model_ui_path
    fi
    # set default template type for csghub
    grep -q "elem_id='template_type',value=" $model_ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='template_type'|elem_id='template_type',value='${template_type_name}'|" $model_ui_path
    fi
    # set default model id for csghub
    grep -q "elem_id='model_id_or_path',value=" $model_ui_path
    if [ $? -ne 0 ]; then
        sed -i "s|elem_id='model_id_or_path'|elem_id='model_id_or_path',value='${REPO_ID}'|" $model_ui_path
        sed -i "s|default='AUTO'|default=None|1" $argument_path
    fi
fi


export GRADIO_ROOT_PATH="${CONTEXT_PATH}/proxy/7860"


ascend_env=/usr/local/Ascend/ascend-toolkit/set_env.sh
if [ -f "$ascend_env" ]; then
    source $ascend_env
    ASCEND_VISIBLE_DEVICES=0 SWIFT_UI_LANG=en swift web-ui
else
    CUDA_VISIBLE_DEVICES=0 SWIFT_UI_LANG=en swift web-ui
fi

