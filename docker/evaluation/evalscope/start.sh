#!/bin/bash

download_model() {
    modelID=$1
    revision=$2
    python /etc/csghub/download.py models --model_ids $modelID --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source csg
    if [ $? -ne 0 ]; then
        echo "Download model $modelID failed."
        exit 1
    fi
}
get_subset_and_task() {
    repo=$1
    csv_file=$(find $repo -name "*_val.csv" -type f | head -n 1)
    tsv_file=$(find $repo -name "*.tsv" -type f | head -n 1)
    jsonl_files=$(find $repo -name "*.jsonl" -type f)
    if [ -n "$csv_file" ]; then
        basename=$(basename "$csv_file")
        star_value="${basename%_val.csv}"
        echo "$star_value|general_mcq"
    elif [ -n "$tsv_file" ]; then
        basename=$(basename "$tsv_file")
        star_value="${basename%.tsv}"
        echo "|CustomRetrieval"
    elif [ -n "$jsonl_files" ]; then
        subset=""
        for jsonl_file in $jsonl_files; do
            basename=$(basename "$jsonl_file")
            star_value="${basename%.jsonl}"
            if [ -z "$subset" ]; then
                subset="\"$star_value\""
            else
                subset="\"$subset\",\"$star_value\""
            fi
        done
        echo  "$subset|general_qa"
    else
        echo "No valid subset found for $repo"
        exit 1
    fi

}
export HF_TOKEN=$ACCESS_TOKEN
mkdir -p /workspace/data

# Register custom datasets
echo "Registering custom datasets..."
python /etc/csghub/register_custom.py

# download datasets
IFS=',' read -r -a dataset_repos <<< "$DATASET_IDS"
IFS=',' read -r -a dataset_revisions <<< "$DATASET_REVISIONS"
# Loop through the array and print each value
dataset_tasks=""
dataset_tasks_args=""
for index in "${!dataset_repos[@]}"; do
    repo=${dataset_repos[$index]}
    revision=${dataset_revisions[$index]}
    # check $dataset existing
    echo "Start downloading dataset $repo..."
    python /etc/csghub/download.py datasets --dataset_ids $repo --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source ms
    if [ $? -ne 0 ]; then
        echo "Download dataset $repo failed,retry with HF mirror"
        #for some special case which use main branch
        python /etc/csghub/download.py datasets --dataset_ids $repo --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source hf
    fi
    if [ "$USE_CUSTOM_DATASETS" = "true" ]; then
        task=$(get_subset_and_task $repo)
        if [ $? -ne 0 ]; then
            echo "Get subset and task for dataset $repo failed."
            exit 1
        fi
        subset=$(echo $task | cut -d '|' -f 1)
        repo_task=$(echo $task | cut -d '|' -f 2)
        echo "Found subset: $subset, task: $repo_task for dataset $repo"
        dataset_tasks="$dataset_tasks $repo_task"
        if [ -z "$dataset_tasks_args" ]; then
            if [ -z "$subset" ]; then
                dataset_tasks_args="\"$repo_task\": {\"local_path\": \"/workspace/$repo\"}"
            else
                dataset_tasks_args="\"$repo_task\": {\"local_path\": \"/workspace/$repo\",\"subset_list\":[$subset]}"
            fi
        else
            if [ -z "$subset" ]; then
                dataset_tasks_args="$dataset_tasks_args,\"$repo_task\": {\"local_path\": \"/workspace/$repo\"}"
            else
                dataset_tasks_args="$dataset_tasks_args,\"$repo_task\": {\"local_path\": \"/workspace/$repo\",\"subset_list\":[$subset]}"
            fi
        fi
    else
        rm -rf /tmp/task.txt 
        python /etc/csghub/get_task.py $repo
        if [ -f /tmp/task.txt ]; then
            repo_task=`cat /tmp/task.txt`
            if [ ! -z "$repo_task" ]; then
                if [ -z "$dataset_tasks" ]; then
                    dataset_tasks="$repo_task"
                else 
                    dataset_tasks="$dataset_tasks $repo_task"
                fi
                if [ -z "$dataset_tasks_args" ]; then
                    dataset_tasks_args="\"$repo_task\": {\"local_path\": \"/workspace/$repo\"}"
                else
                    dataset_tasks_args="$dataset_tasks_args,\"$repo_task\": {\"local_path\": \"/workspace/$repo\"}"
                fi
                
            fi
        fi
    fi 
done
dataset_tasks_args="{${dataset_tasks_args}}"
# start evaluation
if [ -z "$dataset_tasks" ]; then
    echo "dataset_tasks is empty for dataset $DATASET_IDS"
    exit 1
fi
echo "Running tasks: $dataset_tasks, args: $dataset_tasks_args"
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
#LimitedMaxToken is gpu_num multiplied by 4096
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
    # Use wrapper script to ensure custom datasets are registered
    python /etc/csghub/evalscope_wrapper.py eval --model /workspace/$modelID  --datasets $dataset_tasks --dataset-args "$dataset_tasks_args" --limit 10
    if [ $? -ne 0 ]; then
        echo "Evaluation failed for model $model_name."
        exit 1
    fi
    json_file=`find /workspace/outputs/**/reports/${model_name}/ -type f -name "*.json" | tr '\n' ' '`
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
echo "python /etc/csghub/upload_files.py summary --file $jsonFiles --tasks $dataset_tasks"
python /etc/csghub/upload_files.py summary --file $jsonFiles --tasks $dataset_tasks
upload_json_file=`ls -d /workspace/output/final/upload.json`
upload_xlsx_file=`ls -d /workspace/output/final/upload.xlsx`
python /etc/csghub/upload_files.py upload "$upload_json_file,$upload_xlsx_file"
output=`cat /tmp/output.txt`
echo "Evaluation output: $output"
echo "finish evaluation for $MODEL_IDS"
