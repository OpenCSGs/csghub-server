#!/bin/bash

search_path_with_most_term() {
    search_term=$1
    paths=("${@:2}")

    declare -A count_dict
    for path in "${paths[@]}"; do
        count_dict["$path"]=$(grep -o "$search_term" "$path" | wc -l)
    done

    max_count_path=""
    max_count=0
    for path in "${!count_dict[@]}"; do
        count=${count_dict[$path]}
        if [[ $count -gt $max_count ]]; then
            max_count=$count
            max_count_path=$path
        fi
    done
    if [ -z "$max_count_path" ]; then
        echo $paths[0]
        return 0
    fi
    echo $max_count_path
    return 0
}
export HF_TOKEN=$ACCESS_TOKEN
#download datasets
IFS=',' read -r -a dataset_repos <<< "$DATASET_IDS"
IFS=',' read -r -a dataset_revisions <<< "$DATASET_REVISIONS"
for index in "${!dataset_repos[@]}"; do
    repo=${dataset_repos[$index]}
    revision=${dataset_revisions[$index]}
    echo "Downloading datasets..."
    python /etc/csghub/download.py datasets --dataset_ids $repo --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision
done
if [ $? -ne 0 ]; then
    echo "Failed to download datasets"
    exit 1
fi
if [ $? -ne 0 ]; then
    echo "Failed to download models"
    exit 1
fi

tasks=""
task_dir="/workspace/lm-evaluation-harness/lm_eval/tasks"
IFS=',' read -r -a dataset_repos <<< "$DATASET_IDS"
if [ -z "$NUM_FEW_SHOT" ]; then
    NUM_FEW_SHOT=0
fi
script_dts_array=("allenai/winogrande" "facebook/anli" "aps/super_glue" "Rowan/hellaswag" "nyu-mll/blimp" "EdinburghNLP/orange_sum" "facebook/xnli" "nyu-mll/glue" "openai/gsm8k" "cimec/lambada" "allenai/math_qa" "openlifescienceai/medmcqa" "google-research-datasets/nq_open" "allenai/openbookqa" "google-research-datasets/paws-x" "ybisk/piqa" "community-datasets/qa4mre" "allenai/sciq" "allenai/social_i_qa" "LSDSem/story_cloze" "allenai/swag" "IWSLT/iwslt2017" "wmt/wmt14" "wmt/wmt16","mandarjoshi/trivia_qa" "truthfulqa/truthful_qa" "Stanford/web_questions" "ErnestSDavis/winograd_wsc" "cambridgeltl/xcopa" "google/xquad")
script_dts_multi_config_array=("allenai/winogrande")
for repo in "${dataset_repos[@]}"; do
    repo_name="${repo#*/}"
    if [ "$USE_CUSTOM_DATASETS" = "true" ]; then
        task_file=$(find /workspace/$repo -name "*.yaml" -type f | head -n 1)
        
        if [ -z "$task_file" ]; then
            echo "No task file found in custom dataset $repo"
            exit 1
        fi
        mkdir -p /workspace/lm-evaluation-harness/lm_eval/tasks/my-custom-tasks
        cp $task_file /workspace/lm-evaluation-harness/lm_eval/tasks/my-custom-tasks/
    fi

    if [[ " ${script_dts_array[@]} " =~ " ${repo} " ]]; then
        #need replace with real path
        echo "replace script repo with namespace repo"
        find $task_dir -type f -exec sed -i "s|dataset_path: $repo_name|dataset_path: $repo|g" {} +
        if [[ " ${script_dts_multi_config_array[@]} " =~ " ${repo} " ]]; then
            grep -rl "dataset_path: $repo" "$task_dir" | xargs sed -i "s|dataset_name: .*|dataset_name: null|g"
        fi
    else
        find $task_dir -type f -exec sed -i "s|dataset_path: .*${repo_name}|dataset_path: $repo|g" {} +
    fi
    # search full id to cover mirror repo id
    mapfile -t yaml_files < <(grep -Rl -E "(dataset_path: ${repo}($|\s))" $task_dir)
    file_count=${#yaml_files[@]}
    if [ "$file_count" -eq 0 ]; then
        # search short id to cover csghub repo id
        mapfile -t yaml_files < <(grep -Rl -E "(dataset_path: .*/${repo_name}($|\s))|(dataset_path: ${repo_name}($|\s))" $task_dir)
    fi
    file_count=${#yaml_files[@]}
    if [ "$file_count" -eq 0 ]; then
        echo "no yaml file found for repo $repo"
        continue
    fi
    # check yaml_files size
    common_path="${yaml_files[0]}"
    if [ "$file_count" -gt 1 ]; then
        for path in "${yaml_files[@]}"; do
            while [[ "$path" != "${common_path}"* ]]; do
                common_path="${common_path%/*}"
            done
        done
        if [ "x$common_path" == "x$task_dir" ]; then
            echo "no common path found for repo $repo, will pick one of the yaml_files"
            matched_path=$(search_path_with_most_term "$repo_name" "${yaml_files[@]}")
            common_path=$(dirname "$matched_path")
        fi
    else
        common_path=$(dirname "$common_path")
    fi
    #fix sub task from groups
    sub_group_task_yaml=""
    # pick file if contains common path
    for yaml_file in "${yaml_files[@]}"; do
        if [[ "$yaml_file" == "$common_path"* ]]; then
            sub_group_task_yaml="$yaml_file"
            break
        fi
    done

    echo "common path found for repo $repo: $common_path"
    rm -rf /tmp/task.txt
    python /etc/csghub/get_task.py task --task_dir $common_path --sub_task_yaml $sub_group_task_yaml
    if [ -f /tmp/task.txt ]; then
        repo_task=`cat /tmp/task.txt`
        if [ ! -z "$repo_task" ]; then
            tasks="$tasks,$repo_task"
        fi
    fi 
    
done
tasks=$(echo "$tasks" | sed 's/^,//; s/,$//')
tasks=$(echo "$tasks" | tr -d ' ' | tr ',' ',')
echo "will start tasks: $tasks"
IFS=',' read -r -a model_repos <<< "$MODEL_IDS"
modelNames=""
jsonFiles=""
IFS=',' read -r -a model_repos <<< "$MODEL_IDS"
IFS=',' read -r -a model_revisions <<< "$REVISIONS"
for index in "${!model_repos[@]}"; do
    modelID=${model_repos[$index]}
    revision=${model_revisions[$index]}
    echo "Start downloading model $modelID"
    python /etc/csghub/download.py models --model_ids $modelID --endpoint $HF_ENDPOINT --token $HF_TOKEN --revision $revision --source csg
    model_name=`basename $modelID`
    modelNames="$modelNames,$model_name"
    accelerate launch -m lm_eval \
            --model hf \
            --model_args pretrained=${modelID},dtype=auto,trust_remote_code=True \
            --tasks "$tasks,$tasks" \
            --batch_size auto \
            --output_path /workspace/output/

    if [ $? -eq 0 ]; then
        echo "Evaluation completed successfully."
    else
        echo "Evaluation failed."
        exit 1
    fi
    json_file=`ls -dt /workspace/output/*${model_name}*/*.json | head -n 1`
    jsonFiles="$jsonFiles,$json_file"
    rm -rf /workspace/$modelID
done

# upload result to mino server
mkdir -p /workspace/output/final
python /etc/csghub/upload_files.py summary --file $jsonFiles --model $modelNames
upload_json_file=`ls -d /workspace/output/final/upload.json`
upload_xlsx_file=`ls -d /workspace/output/final/upload.xlsx`
python /etc/csghub/upload_files.py upload "$upload_json_file,$upload_xlsx_file"
output=`cat /tmp/output.txt`
echo "Evaluation output: $output"
echo "finish evaluation for $MODEL_IDS"