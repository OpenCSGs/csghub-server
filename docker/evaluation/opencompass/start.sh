#!/bin/bash

download_model() {
    modelID=$1
    revision=$2
    python /etc/csghub/download.py models --model_ids $modelID --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source csg
    if [ $? -ne 0 ]; then
        echo "Download model $modelID failed."
        exit 1
    fi
    insert_string=$(cat << 'EOF'
"chat_template":"{%- if messages[0]['role'] == 'system' -%}{%- set system_message = messages[0]['content'] -%}{%- set messages = messages[1:] -%}{%- else -%}{% set system_message = '' -%}{%- endif -%}{{ bos_token + system_message }}{%- for message in messages -%}{%- if (message['role'] == 'user') != (loop.index0 % 2 == 0) -%}{{ raise_exception('Conversation roles must alternate user/assistant/user/assistant/...') }}{%- endif -%}{%- if message['role'] == 'user' -%}{{ 'USER: ' + message['content'] + '\\n' }}{%- elif message['role'] == 'assistant' -%}{{ 'ASSISTANT: ' + message['content'] + eos_token + '\\n' }}{%- endif -%}{%- endfor -%}{%- if add_generation_prompt -%}{{ 'ASSISTANT:' }}{% endif %}",
EOF
)

    repo_tokenizer_config="/workspace/$modelID/tokenizer_config.json"
    # check if repo_tokenizer_config exists
    if [ ! -f "$repo_tokenizer_config" ]; then
        echo "the model is invalid."
        exit 1
    fi

    # fix some model does not contain chat_template
    if ! grep -q "chat_template" "$repo_tokenizer_config"; then
        filename="/tmp/tokenizer_config.json"
        cp "/workspace/$modelID/tokenizer_config.json" $filename
        awk -v ins="$insert_string" '/tokenizer_class/ {print; print ins; next}1' "$filename" > tmpfile && mv -f tmpfile $repo_tokenizer_config
    fi
    repo_config="/workspace/$modelID/config.json"
    sed -i "s/\"max_position_embeddings\": [0-9]*/\"max_position_embeddings\": $LimitedMaxToken/" $repo_config
}

export HF_TOKEN=$ACCESS_TOKEN
mkdir -p /workspace/data
ANSWER_MODE=${ANSWER_MODE:-"gen"}
# download model

#fix: use local dataset
export DATASET_SOURCE=ModelScope
export COMPASS_DATA_CACHE=/workspace/data/
dataset_path="/usr/local/lib/python3.10/dist-packages/opencompass/datasets/"
if [ ! -e "/workspace/.init" ]; then
    find $dataset_path -type f -name "*.py" -exec sed -i 's/get_data_path(path)/"\/workspace\/data\/"+get_data_path(path)/g' {} +
    touch /workspace/.init
fi
declare -A dataset_alias
dataset_alias["ai2_arc"]="ARC-c"
dataset_alias["ceval-exam"]="ceval"
dataset_alias["OCNLI"]="ocnli"
dataset_alias["cmrc_dev"]="CMRC_dev"
dataset_alias["drcd_dev"]="DRCD_dev"
dataset_alias["humaneval"]="openai_humaneval"
dataset_alias["LCSTS"]="lcsts"
dataset_alias["natural_question"]="nq"
dataset_alias["strategy_qa"]="strategyqa"
dataset_alias["boolq"]="BoolQ"
dataset_alias["trivia_qa"]="triviaqa"
dataset_alias["xsum"]="Xsum"
# download datasets
IFS=',' read -r -a dataset_repos <<< "$DATASET_IDS"
IFS=',' read -r -a dataset_revisions <<< "$DATASET_REVISIONS"
# Loop through the array and print each value
dataset_tasks=""
dataset_tasks_ori=""
for index in "${!dataset_repos[@]}"; do
    repo=${dataset_repos[$index]}
    revision=${dataset_revisions[$index]}
    # check $dataset existing
    python /etc/csghub/download.py datasets --dataset_ids $repo --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source ms
    if [ $? -ne 0 ]; then
        echo "Download dataset $repo failed,retry with HF mirror"
        #for some special case which use main branch
        python /etc/csghub/download.py datasets --dataset_ids $repo --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source hf
    fi
    # if custom datasets, use the first csv or json file, and skip the rest
    if [ "$USE_CUSTOM_DATASETS" = "true" ]; then
        data_file_path=($(find /workspace/data/$repo -type f \( -name "*.csv" -o -name "*.jsonl" \) | head -n 1))
        custom_datasets_path=$data_file_path
        FILE_NAME="${data_file_path##*/}"
        task_name="${FILE_NAME%.*}"
        dataset_tasks="custom_dataset"
        dataset_tasks_ori=$task_name
        continue 
    fi
    # get answer mode
    task_path=`python -W ignore /etc/csghub/get_answer_mode.py $repo`
    if [ -z "$task_path" ]; then
        echo "task_path is empty for dataset $repo"
        exit 1
    fi
    datasets_conf_dir="/usr/local/lib/python3.10/dist-packages/opencompass/configs/datasets/"
    mapfile -t dataset_conf_files < <(find $datasets_conf_dir -type f -name "*.py" -exec grep -rl "'$task_path'" {} + )
    if [ -z "$dataset_conf_files" ]; then
        echo "Cannot find dataset config location for $task_path"
        exit 1
    fi
    #loop dataset_conf_files
    for dataset_conf_file in "${dataset_conf_files[@]}"; do
        dataset_conf_dir=`dirname $dataset_conf_file`
        task_conf_file=`find $dataset_conf_dir -type f -name "*$ANSWER_MODE.py" | head -n 1`
        if [ -n "$task_conf_file" ]; then
            break
        fi
    done
    if [ -n "$task_conf_file" ]; then
        task=`basename $task_conf_file | cut -d'.' -f1`
        dataset_tasks="$dataset_tasks $task"
        ori_name=`basename $repo`
        if [ -n "${dataset_alias[$ori_name]}" ]; then
            ori_name="${dataset_alias[$ori_name]}"
        fi
        dataset_tasks_ori="$dataset_tasks_ori $ori_name"
        continue
    fi
done
# start evaluation
if [ -z "$dataset_tasks" ]; then
    echo "dataset_tasks is empty for dataset $DATASET_IDS"
    exit 1
fi
echo "Running tasks: $dataset_tasks, custom datasets: $custom_datasets_path"
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
#LimitedMaxToken is gpu_num multiplied by 4096
LimitedMaxToken=$(($GPU_NUM * 4096))
jsonFiles=""
IFS=',' read -r -a model_repos <<< "$MODEL_IDS"
IFS=',' read -r -a model_revisions <<< "$REVISIONS"
for index in "${!model_repos[@]}"; do
    modelID=${model_repos[$index]}
    revision=${model_revisions[$index]}
    download_model $modelID $revision
    if [ $? -ne 0 ]; then
        echo "Download model $modelID failed."
        exit 1
    fi
    model_name=`basename $modelID`
    echo "Start evaluating model $model_name, dataset $dataset_tasks"
    if [ "$USE_CUSTOM_DATASETS" = "true" ]; then
        opencompass --custom-dataset-path $custom_datasets_path  --work-dir /workspace/output  --hf-type chat --hf-path /workspace/$modelID -a vllm --max-out-len 100 --max-seq-len $LimitedMaxToken --batch-size 8 --hf-num-gpus $GPU_NUM --max-num-workers $GPU_NUM --work-dir /workspace/output/$modelID
    else
        opencompass --datasets $dataset_tasks --work-dir /workspace/output  --hf-type chat --hf-path /workspace/$modelID -a vllm --max-out-len 100 --max-seq-len $LimitedMaxToken --batch-size 8 --hf-num-gpus $GPU_NUM --max-num-workers $GPU_NUM --work-dir /workspace/output/$modelID
    fi
    if [ $? -ne 0 ]; then
        echo "Evaluation failed for model $model_name."
        exit 1
    fi
    csv_file=`ls -dt /workspace/output/$modelID/**/summary/*.csv |head -n 1`
    python /etc/csghub/upload_files.py convert "$csv_file"
    json_file=`ls -dt /workspace/output/$modelID/**/summary/*.json | head -n 1`
    jsonFiles="$jsonFiles $json_file"
    # remove model to save space
    rm -rf /workspace/$modelID
done

if [ $? -eq 0 ]; then
    echo "Evaluation completed successfully."
else
    echo "Evaluation failed."
    exit 1
fi

# upload result to mino server
mkdir -p /workspace/output/final
echo "python /etc/csghub/upload_files.py summary --file $jsonFiles --tasks $dataset_tasks_ori"
python /etc/csghub/upload_files.py summary --file $jsonFiles --tasks $dataset_tasks_ori
upload_json_file=`ls -d /workspace/output/final/upload.json`
upload_xlsx_file=`ls -d /workspace/output/final/upload.xlsx`
python /etc/csghub/upload_files.py upload "$upload_json_file,$upload_xlsx_file"
output=`cat /tmp/output.txt`
echo "Evaluation output: $output"
echo "finish evaluation for $MODEL_IDS"
