#!/bin/bash
mkdir -p /workspace/data
ANSWER_MODE=${ANSWER_MODE:-"gen"}
# download model
csghub-cli download $MODEL_ID -k $ACCESS_TOKEN -e $HF_ENDPOINT -cd /workspace/
insert_string=$(cat << 'EOF'
"chat_template":"{%- if messages[0]['role'] == 'system' -%}{%- set system_message = messages[0]['content'] -%}{%- set messages = messages[1:] -%}{%- else -%}{% set system_message = '' -%}{%- endif -%}{{ bos_token + system_message }}{%- for message in messages -%}{%- if (message['role'] == 'user') != (loop.index0 % 2 == 0) -%}{{ raise_exception('Conversation roles must alternate user/assistant/user/assistant/...') }}{%- endif -%}{%- if message['role'] == 'user' -%}{{ 'USER: ' + message['content'] + '\\n' }}{%- elif message['role'] == 'assistant' -%}{{ 'ASSISTANT: ' + message['content'] + eos_token + '\\n' }}{%- endif -%}{%- endfor -%}{%- if add_generation_prompt -%}{{ 'ASSISTANT:' }}{% endif %}",
EOF
)
repo_tokenizer_config="/workspace/$MODEL_ID/tokenizer_config.json"
# fix some model does not contain chat_template
if ! grep -q "chat_template" "$repo_tokenizer_config"; then
    filename="/tmp/tokenizer_config.json"
    cp "/workspace/$MODEL_ID/tokenizer_config.json" $filename
    awk -v ins="$insert_string" '/tokenizer_class/ {print; print ins; next}1' "$filename" > tmpfile && mv -f tmpfile $repo_tokenizer_config
fi
# download datasets
IFS=',' read -r -a dataset_repos <<< "$DATASET_IDS"
# Loop through the array and print each value
dataset_tasks=""
dataset_tasks_ori=""
for repo in "${dataset_repos[@]}"; do
    # check $dataset existing
    if [ ! -d "/workspace/data/$repo" ]; then
        echo "Start downloading dataset $repo..."
        csghub-cli download $repo -t dataset -k $ACCESS_TOKEN -e $HF_ENDPOINT -cd /tmp/
        # mv "$dataset" to "/workspace/data/"
        mv -f "/tmp/$repo" "/workspace/data/"
    fi
    # get answer mode
    task_path=`python -W ignore /etc/csghub/get_answer_mode.py $repo`
    if [ -z "$task_path" ]; then
        echo "task_path is empty for dataset $repo"
        exit 1
    fi
    datasets_conf_dir="/usr/local/lib/python3.10/dist-packages/opencompass/configs/datasets/"
    dataset_conf_file=`find $datasets_conf_dir -type f -name "*.py" -exec grep -rl "'$task_path'" {} + | head -n 1`
    if [ -z "$dataset_conf_file" ]; then
        echo "Cannot find dataset config location for $task_path"
        exit 1
    fi
    #get dataset_location dir
    dataset_conf_dir=`dirname $dataset_conf_file`
    task_conf_file=`find $dataset_conf_dir -type f -name "*$ANSWER_MODE.py" | head -n 1`
    if [ -n "$task_conf_file" ]; then
        task=`basename $task_conf_file | cut -d'.' -f1`
        dataset_tasks="$dataset_tasks $task"
        ori_name=`basename $repo`
        dataset_tasks_ori="$dataset_tasks_ori $ori_name"
        continue
    fi
done
# start evaluation
if [ -z "$dataset_tasks" ]; then
    echo "dataset_tasks is empty for dataset $DATASET_IDS"
    exit 1
fi
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
#LimitedMaxToken is gpu_num multiplied by 4096
LimitedMaxToken=$(($GPU_NUM * 4096))

opencompass --datasets $dataset_tasks --work-dir /workspace/output  --hf-type chat --hf-path /workspace/$MODEL_ID -a vllm --max-out-len 100 --max-seq-len $LimitedMaxToken --batch-size 8 --hf-num-gpus $GPU_NUM --max-num-workers $GPU_NUM 

# upload result to mino server
output_dir=`ls -dt /workspace/output/* |head -n 1`
csv_file=`ls -d $output_dir/summary/*.csv`
python /etc/csghub/upload_files.py convert "$csv_file"
json_file=`ls -d $output_dir/summary/*.json`
python /etc/csghub/upload_files.py summary --file $json_file --tasks $dataset_tasks_ori
upload_json_file=`ls -d $output_dir/summary/*upload.json`
upload_xlsx_file=`ls -d $output_dir/summary/*upload.xlsx`
python /etc/csghub/upload_files.py upload "$upload_json_file,$upload_xlsx_file"
echo "finish evaluation for $MODEL_ID"
