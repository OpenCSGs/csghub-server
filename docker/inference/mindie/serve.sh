#!/bin/bash


model_name="$REPO_ID"
model_weight_path="/workspace/$REPO_ID"
npu_mem_size=8
world_size=$NPU_NUM
if [  -z "$world_size" ]; then
    world_size=1
fi

echo "model_name: $model_name, model_weight_path: $model_weight_path, world_size: $world_size ,npu_mem_size: $npu_mem_size"


npuids=""
card_num=$(($world_size - 1))
for i in `seq 0 $card_num`
    do
        if [[ $i  == $card_num ]] ;
        then
            npuids=$npuids$i
        else
            npuids=$npuids$i","
        fi    
    done


echo $npuids 

DEPLOYMENT_CONF_PATH="/usr/local/Ascend/mindie/latest/mindie-service/conf/config.json"

cat <<EOF > $DEPLOYMENT_CONF_PATH
{
    "OtherParam" :
    {
        "ResourceParam" :
        {
            "cacheBlockSize" : 128
        },
        "LogParam" :
        {
            "logLevel" : "Info",
            "logPath" : "/workspace/mindservice.log"
        },
        "ServeParam" :
        {
            "ipAddress" : "127.0.0.1",
            "port" : 8000,
            "managementPort" : 1026,
            "maxLinkNum" : 10,
            "httpsEnabled" : false,
            "tlsCaPath" : "security/ca/",
            "tlsCaFile" : ["ca.pem"],
            "tlsCert" : "security/certs/server.pem",
            "tlsPk" : "security/keys/server.key.pem",
            "tlsPkPwd" : "security/pass/mindie_server_key_pwd.txt",
            "tlsCrl" : "security/certs/server_crl.pem",
            "managementTlsCaFile" : ["management_ca.pem"],
            "managementTlsCert" : "security/certs/management_server.pem",
            "managementTlsPk" : "security/keys/management_server.key.pem",
            "managementTlsPkPwd" : "security/pass/management_mindie_server_key_pwd.txt",
            "managementTlsCrl" : "security/certs/management_server_crl.pem",
            "kmcKsfMaster" : "tools/pmt/master/ksfa",
            "kmcKsfStandby" : "tools/pmt/standby/ksfb",
            "multiNodesInferPort" : 1120,
            "interNodeTLSEnabled" : true,
            "interNodeTlsCaFile" : "security/ca/ca.pem",
            "interNodeTlsCert" : "security/certs/server.pem",
            "interNodeTlsPk" : "security/keys/server.key.pem",
            "interNodeTlsPkPwd" : "security/pass/mindie_server_key_pwd.txt",
            "interNodeKmcKsfMaster" : "tools/pmt/master/ksfa",
            "interNodeKmcKsfStandby" : "tools/pmt/standby/ksfb"
        }
    },
    "WorkFlowParam" :
    {
        "TemplateParam" :
        {
            "templateType" : "Standard",
            "templateName" : "Standard_llama"
        }
    },
    "ModelDeployParam" :
    {
        "engineName" : "mindieservice_llm_engine",
        "modelInstanceNumber" : 1,
        "tokenizerProcessNumber" : 8,
        "maxSeqLen" : 2560,
        "npuDeviceIds" : [[$npuids]],
        "multiNodesInferEnabled" : false,
        "ModelParam" : [
            {
                "modelName" : "$model_name",
                "modelWeightPath" : "$model_weight_path",
                "worldSize" : $world_size,
                "cpuMemSize" : 5,
                "npuMemSize" : $npu_mem_size,
                "backendType": "atb",
                "pluginParams" : ""
            }
        ]
    },
    "ScheduleParam" :
    {
        "maxPrefillBatchSize" : 50,
        "maxPrefillTokens" : 8192,
        "prefillTimeMsPerReq" : 150,
        "prefillPolicyType" : 0,

        "decodeTimeMsPerReq" : 50,
        "decodePolicyType" : 0,

        "maxBatchSize" : 200,
        "maxIterTimes" : 512,
        "maxPreemptCount" : 0,
        "supportSelectBatch" : true,
        "maxQueueDelayMicroseconds" : 5000
    }
}
EOF
cat $DEPLOYMENT_CONF_PATH

source /usr/local/Ascend/ascend-toolkit/set_env.sh
source /usr/local/Ascend/nnal/atb/set_env.sh
source /usr/local/Ascend/mindie/set_env.sh
source /usr/local/Ascend/llm_model/set_env.sh

# download model
python3 /etc/csghub/entry.py

# conver model if no safetensor file in model_weight_path
if find $model_weight_path -type f -name "*.safetensors" | grep -q .; then
    echo "No need convert model to safetensors"
else
    echo "start convert model to safetensors"
    echo -e "y\ny\n" | python /usr/local/Ascend/llm_model/examples/convert/convert_weights.py --model_path $model_weight_path
fi

# install mini transformer 
VERSION=`grep 'transformers_version' $model_weight_path/config.json | sed 's/.*"transformers_version": "\(.*\)".*/\1/'`
if [[ $(printf '%s\n' "4.33.3" "$VERSION" | sort -V | head -n 1) == "$VERSION" ]]; then
    echo "no need upgrade transformers"
else
    pip install transformers==$VERSION
fi
congfile="$model_weight_path/config.json"
# qwen has no torch_dtype
if ! grep -q "torch_dtype" "$congfile"; then
    insert_string='"torch_dtype": "float16",'
    awk -v ins="$insert_string" '/tokenizer_class/ {print; print ins; next}1' "$congfile" > tmpfile && mv tmpfile "$congfile"
fi

# use float16 type to infernece as mindie don't support float32
sed -i 's/"torch_dtype": "float32"/"torch_dtype": "float16"/' $congfile
sed -i 's/"torch_dtype": "bfloat16"/"torch_dtype": "float16"/' $congfile

export OPENBLAS_NUM_THREADS=2
export MIES_PYTHON_LOG_TO_FILE=1
export MIES_PYTHON_LOG_TO_STDOUT=1
export PYTHONPATH=/usr/local/Ascend/llm_model:$PYTHONPATH
export PATH=$PATH:/usr/local/Ascend/mindie/latest/mindie-service/bin
mindieservice_daemon