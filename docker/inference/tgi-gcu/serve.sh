#!/bin/bash
unset HF_HUB_OFFLINE
export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi

VISIBLE_DEVICES=""
card_num=$(($GPU_NUM - 1))
for i in `seq 0 $card_num`
    do
        if [[ $i  == $card_num ]] ;
        then
            VISIBLE_DEVICES=$VISIBLE_DEVICES$i
        else
            VISIBLE_DEVICES=$VISIBLE_DEVICES$i","
        fi    
    done


export TOPS_VISIBLE_DEVICES=$VISIBLE_DEVICES
export HF_TOKEN="$ACCESS_TOKEN"
if [[ "$REPO_ID" == *"1.5B"* ]]; then
    export TOPS_LIB_PATH="/usr/lib/libtopstransformer_1.5B.so"
elif [[ "$REPO_ID" == *"7B"* ]]; then
    export TOPS_LIB_PATH="/usr/lib/libtopstransformer.so"
elif [[ "$REPO_ID" == *"14B"* ]]; then
    export TOPS_LIB_PATH="/usr/lib/libtopstransformer_14b.so"
elif [[ "$REPO_ID" == *"32B"* ]]; then
    export TOPS_LIB_PATH="/usr/lib/libtopstransformer_32b.so"
elif [[ "$REPO_ID" == *"70B"* ]]; then
    export TOPS_LIB_PATH="/usr/lib/libtopstransformer_70b.so"
else
    echo "REPO_ID does not match any specific pattern."
    exit 1
fi
# for qwen model
export GCU_SUPPORT_MODEL_NAME="qwen"
export TOPS_USE_TP=True
ori_model_path="/workspace/$REPO_ID-ori"
convert_path="/workspace/$REPO_ID"
Args="--max-concurrent-requests=1024 --max-input-length=4096 --max-total-tokens=5120 --max-batch-prefill-tokens=5120"
if [[ "$REPO_ID" == *"DeepSeek-R1-Distill-Qwen-1.5B"* ]]; then
    export TOPS_USE_TP=False
    export GCU_SUPPORT_MODEL_NAME="wukong"
    export TOPS_INI_PATH="/usr/local/gcu/sample/topstransformer/model_demo/csg-wukong/ds-distill-qwen_1.5b.ini"
    cd /usr/local/gcu/sample/topstransformer/model_demo/csg-wukong/
    python3 weight_preprocess_csg-wukong.py -i ${ori_model_path} -o ${convert_path} -cfg ${ori_model_path}/config.json
    Args="--max-concurrent-requests=1024 --max-input-length=7168 --max-total-tokens=8192  --max-batch-prefill-tokens=8192"
elif [[ "$REPO_ID" == *"DeepSeek-R1-Distill-Qwen-7B"* ]]; then
    export TOPS_INI_PATH="/usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/qwen2_7b_bf16.ini"
    cd /usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/
    python3 weight_preprocess_qwen2.5_7b.py -tp $GPU_NUM -i ${ori_model_path} -o ${convert_path} -cfg ${ori_model_path}/config.json
    Args="--max-concurrent-requests=1024 --max-input-length=7168 --max-total-tokens=8192  --max-batch-prefill-tokens=8192"
elif [[ "$REPO_ID" == *"DeepSeek-R1-Distill-Qwen-14B"* ]]; then
    export TOPS_INI_PATH="/usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/qwen2.5_14b_fp16.ini"
    cd /usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/
    python3 weight_preprocess_qwen2.5_14b.py -tp $GPU_NUM -i ${ori_model_path} -o ${convert_path} -cfg ${ori_model_path}/config.json
elif [[ "$REPO_ID" == *"DeepSeek-R1-Distill-Qwen-32B"* ]]; then
    export TOPS_USE_TP=$GPU_NUM
    export TOPS_MP=true
    export TOPS_INI_PATH="/usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/qwen1.5_32b.ini"
    cd /usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/
    python3 weight_preprocess_qwen1.5_32b.py -tp $GPU_NUM -i ${ori_model_path} -o ${convert_path} -cfg ${ori_model_path}/config.json
elif [[ "$REPO_ID" == *"DeepSeek-R1-Distill-Llama-70B"* ]]; then
    pip install transformers==4.43.1
    export TOPS_MP=true
    export GCU_SUPPORT_MODEL_NAME="qwen"
    export TOPS_USE_TP=$GPU_NUM
    export TOPS_INI_PATH="/usr/local/gcu/sample/topstransformer/model_demo/LLaMa3/ds-distill-llama_70b.ini"
    cd /usr/local/gcu/sample/topstransformer/model_demo/qwen1.5/
    python3 weight_preprocess_llama_70b.py -tp $GPU_NUM -i ${ori_model_path} -o ${convert_path} -cfg ${ori_model_path}/config.json
    Args="--max-concurrent-requests=1024 --max-input-length=7168 --max-total-tokens=8192  --max-batch-prefill-tokens=8192"
    pip install transformers==4.40.0
fi
cp -rf ${ori_model_path}/*.json ${convert_path}/
rm -rf ${ori_model_path}
cd /workspace/

export TOPS_TRANSFORMER_BATCH=1


text-generation-launcher --model-id "$REPO_ID" --num-shard=1 --port $PORT --trust-remote-code --disable-custom-kernels --waiting-served-ratio=0.1 --max-waiting-tokens=6 $Args
