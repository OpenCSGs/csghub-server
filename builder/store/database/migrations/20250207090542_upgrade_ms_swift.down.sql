SET statement_timeout = 0;

--bun:split

DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'SmallThinker-3B-Preview';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'internlm3-8b-instruct';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Zero';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Qwen-1.5B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Qwen-7B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Qwen-14B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Qwen-32B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Llama-8B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'DeepSeek-R1-Distill-Llama-70B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'phi-4';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'MiniMax-Text-01';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'MiniCPM-V-2_6';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'MiniCPM-o-2_6';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'MiniMax-VL-01';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-7B-Instruct-1M';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-14B-Instruct-1M';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'UI-TARS-2B-SFT';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'UI-TARS-7B-SFT';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'UI-TARS-7B-DPO';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'UI-TARS-72B-SFT';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'UI-TARS-72B-DPO';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-VL-3B-Instruct';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-VL-7B-Instruct';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-VL-72B-Instruct';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Janus-Pro-1B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Janus-Pro-7B';
DELETE FROM resource_models WHERE engine_name = 'ms-swift' AND model_name = 'Qwen2.5-Math-7B-PRM800K';
