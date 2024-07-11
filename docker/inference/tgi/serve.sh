#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
cp "/workspace/$REPO_ID/tokenizer_config.json" /etc/csghub/tokenizer_config.json
filename="/etc/csghub/tokenizer_config.json"
insert_string=$(cat << 'EOF'
"chat_template": "{% if messages[0]['role'] == 'system' %}{% set loop_messages = messages[1:] %}{% set system_message = messages[0]['content'] %}{% else %}{% set loop_messages = messages %}{% set system_message = false %}{% endif %}{% for message in loop_messages %}{% if (message['role'] == 'user') != (loop.index0 % 2 == 0) %}{{ raise_exception('Conversation roles must alternate user/assistant/user/assistant/...') }}{% endif %}{% if loop.index0 == 0 and system_message != false %}{% set content = '<<SYS>>\\n' + system_message + '\\n<</SYS>>\\n\\n' + message['content'] %}{% else %}{% set content = message['content'] %}{% endif %}{% if message['role'] == 'user' %}{{ '<s>[INST] ' + content.strip() + ' [/INST]' }}{% elif message['role'] == 'assistant' %}{{ ' '  + content.strip() + ' </s>' }}{% endif %}{% endfor %}",
EOF
)

# fix some model does not contain chat_template
if ! grep -q "chat_template" "$filename"; then
    awk -v ins="$insert_string" '/tokenizer_class/ {print; print ins; next}1' "$filename" > tmpfile && mv tmpfile "$filename"
fi

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi

text-generation-launcher --model-id "$REPO_ID" --tokenizer-config-path "$filename" --num-shard="$GPU_NUM" --port $PORT --trust-remote-code
