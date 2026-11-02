Follow doc of llm-inference project to setup the env.

## start ray cluster
ray start --head --port=6379 --dashboard-host=0.0.0.0 --dashboard-port=8265

## start api server
llm-serve start apiserver


## start a model
curl -H "Content-Type: application/json" -H "user-name: default"  -d '[{"model_id": "gpt2", "model_task": "text-generation", "model_revision": "main", "is_oob": true, "scaling_config": {"num_workers": 0,"num_gpus_per_worker": 1,"num_cpus_per_worker": 1}}]' -X POST "http://127.0.0.1:8000/api/start_serving"

## stop a model
curl -H "Content-Type: application/json" -H "user-name: default"  -d '[{"model_id": "gpt2", "model_task": "text-generation", "model_revision": "main", "is_oob": true, "scaling_config": {"num_workers": 0,"num_gpus_per_worker": 1,"num_cpus_per_worker": 1}}]' -X POST "http://127.0.0.1:8000/api/stop_serving"

## list all running models
curl -H "Content-Type: application/json" -H "user-name: default"  -X GET "http://127.0.0.1:8000/api/list_serving"

## stop api server
llm-serve stop apiserver 

## stop ray cluster
ray stop


