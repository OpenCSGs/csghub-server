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
sed -i "s|USE_HF'|USE_CSGHUB_MODEL'|" $model_path
sed -i "s|USE_HF_TRANSFER|USE_CSGHUB_TRANSFER|" $model_path
sed -i "s|USE_HF|USE_CSGHUB_MODEL|" /etc/csghub/ms-swift/swift/trainers/push_to_ms.py
#change deploy port
sed -i "s|8000|9000|" /etc/csghub/ms-swift/swift/ui/llm_infer/generate.py

template_path="/etc/csghub/ms-swift/swift/llm/utils/template.py"
model_ui_path="/etc/csghub/ms-swift/swift/ui/llm_train/model.py"
hf_model_id=`grep -E "hf_model_id='.*\/$MODEL_NAME'" $model_path | awk -F"'" '{print $2}'`
#find model type and template type
types=`awk -v model_id="${hf_model_id}" '
BEGIN {RS=")"; FS="\n"}
/@register_model/ {
    for(i=1; i<=NF; i++) {
        #reset variable in first line
        if(i==1) {
            modelType = "";
            templateType = "";
            lowerTransformers = "";
        }
        if($i ~ /ModelType\./) {
            split($i, arrModelType, ".");
            modelType = arrModelType[2];
        }
        if($i ~ /TemplateType\./) {
            split($i, arrTemplateType, ".");
            templateType = arrTemplateType[2];
        }
        if($i ~ /transformers</) {
            lowerTransformers = "yes";
        }
        if($i ~ /hf_model_id/ && index($0, "hf_model_id='\''" model_id "'\''")) {
            print modelType templateType lowerTransformers;
            break;
        }
    }
}
' $model_path`
IFS=',' read -ra item_types <<< $types
model_type=${item_types[0]}
template_type=${item_types[1]}
lower_transformers=${item_types[2]}
if [ "x${model_type}" != "x" ]; then
    #replace HF ID
    sed -i "s|hf_model_id='$hf_model_id'|hf_model_id='$REPO_ID'|g" $model_path
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
        sed -i "0,/default='AUTO'/s/default='AUTO'/default=None/" $argument_path
    fi
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

