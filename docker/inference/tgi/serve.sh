#!/bin/bash

ldconfig 2>/dev/null || echo "Unable to refresh the dynamic linker cache; continuing."

export PYTHONPATH="$(pwd):$PYTHONPATH"

# CSGHUB_PYTHON lets an image install the downloader in its own environment
# instead of the one the inference engine runs in.
"${CSGHUB_PYTHON:-python3}" /etc/csghub/entry.py
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

# text-generation-router embeds libpython but lives outside that interpreter,
# so it loads another stdlib. On the ROCm image that is Python 3.11.0rc1, whose
# MetadataPathFinder.invalidate_caches is not a classmethod, and tokenizer import
# crashes. Point the embedded interpreter at the venv base after the downloader:
# the downloader venv is a different Python and cannot start with this PYTHONHOME.
if [ -x /app/.venv/bin/python3 ]; then
    _tgi_python_home="$(/app/.venv/bin/python3 -c 'import sys; print(sys.base_prefix)')"
    if [ -n "$_tgi_python_home" ] && [ "$_tgi_python_home" != /usr ]; then
        export PYTHONHOME="$_tgi_python_home"
    fi
fi

exec text-generation-launcher \
    --model-id "$REPO_ID" \
    --tokenizer-config-path "$filename" \
    --num-shard="$GPU_NUM" \
    --port "$PORT" \
    --trust-remote-code \
    $ENGINE_ARGS
