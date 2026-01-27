#!/bin/bash
set -euo pipefail 

get_env_var() {
    local name="$1"
    local default="${2:-}"
    local required="${3:-true}" 

    local value="${!name:-$default}"
    if [ "$required" = "true" ] && [ -z "$value" ]; then
        echo "ERROR: Environment variable $name is not set, please check configuration!" >&2
        exit 1
    fi
    echo "$value"
}

run_command() {
    local cmd="$1"
    local desc="$2"
    echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S')] === $desc ==="
    if ! eval "$cmd"; then
        echo "[$(date +'%Y-%m-%d %H:%M:%S')] ❌ $desc failed!" >&2
        return 1 
    fi
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ✅ $desc done!"
    return 0
}

SDK=$(get_env_var "SDK" "gradio" "false")
REPO_ID=$(get_env_var "REPO_ID") 
DOWNLOAD_DIR="/home/user/app/${REPO_ID}"
REVISION=$(get_env_var "REVISION" "main" "false") 
TOKEN=$(get_env_var "ACCESS_TOKEN") 
ENDPOINT=$(get_env_var "HF_ENDPOINT" "https://hub.opencsg.com" "false") 
PIP_INDEX_URL=$(get_env_var "PIP_INDEX_URL" "https://mirrors.aliyun.com/pypi/simple/" "false")

REPO_TYPE=$(get_env_var "REPO_TYPE" "space" "false") 
IGNORE_PATTERNS=$(get_env_var "IGNORE_PATTERNS" "*.bin,.venv,venv,.git,.DS_Store,.gitignore,.python-version" "false") 
MAX_WORKERS=$(get_env_var "MAX_WORKERS" "8" "false")  
MAX_RETRIES=$(get_env_var "MAX_RETRIES" "15" "false") 
RETRY_INTERVAL=10 

export CSGHUB_DOMAIN="$ENDPOINT"

echo "[$(date +'%Y-%m-%d %H:%M:%S')] "
echo "  - REPO_ID: $REPO_ID"
echo "  - REPO_TYPE: $REPO_TYPE"
echo "  - DOWNLOAD_DIR: $DOWNLOAD_DIR"
echo "  - REVISION: $REVISION"
echo "  - ENDPOINT: $ENDPOINT"
echo "  - IGNORE_PATTERNS: $IGNORE_PATTERNS"
echo "  - MAX_RETRIES: $MAX_RETRIES"
echo "  - SDK: $SDK"

download_repo() {
    local retry_count=0
    local download_success=false

    local download_cmd=".venv/bin/csghub-cli download \
        ${REPO_ID} \
        --repo-type ${REPO_TYPE} \
        --revision ${REVISION} \
        --endpoint ${ENDPOINT} \
        --token ${TOKEN} \
        --cache-dir ${DOWNLOAD_DIR} \
        --local-dir ${DOWNLOAD_DIR} \
        --ignore-patterns ${IGNORE_PATTERNS} \
        --max-workers ${MAX_WORKERS}"

    while [ $retry_count -lt $MAX_RETRIES ]; do
        echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S')] 📥 Starting repo download (attempt $((retry_count+1))):"
        if run_command "$download_cmd" "download repo ${REPO_ID}"; then
            download_success=true
            break
        fi

        retry_count=$((retry_count+1))
        if [ $retry_count -lt $MAX_RETRIES ]; then
            echo "[$(date +'%Y-%m-%d %H:%M:%S')] ⏳ Download failed, retrying in ${RETRY_INTERVAL} seconds (${MAX_RETRIES - retry_count} attempts left)..."
            sleep $RETRY_INTERVAL
        fi
    done

    if [ "$download_success" = "false" ]; then
        echo "[$(date +'%Y-%m-%d %H:%M:%S')] ❌ Repo ${REPO_ID} download failed after ${MAX_RETRIES} attempts!" >&2
        exit 1
    fi

    if [ ! -d "$DOWNLOAD_DIR" ]; then
        echo "[$(date +'%Y-%m-%d %H:%M:%S')] ❌ Download directory ${DOWNLOAD_DIR} does not exist!" >&2
        exit 1
    fi
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] 📥 Repo ${REPO_ID} download completed successfully!"
}

download_repo

if [ "$SDK" != "nginx" ]; then
    requirements_path="${DOWNLOAD_DIR}/requirements.txt"
    if [ -f "$requirements_path" ]; then
        run_command "uv pip install --no-cache -i $PIP_INDEX_URL -r $requirements_path --python .venv/bin/python " "install dependencies from $requirements_path"
    else
        echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S')] requirements.txt not found, skip installing dependencies."
    fi

    app_path="${DOWNLOAD_DIR}/app.py"
    if [ ! -f "$app_path" ]; then
        echo "ERROR: Application entry file $app_path does not exist!" >&2
        exit 1
    fi
fi

cd "$DOWNLOAD_DIR"
echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S')] 🚀 start application (SDK: $SDK)："
case "$SDK" in
    "gradio")
        run_command "$HOME/app/.venv/bin/python $app_path" "start gradio application"
        ;;
    "streamlit")
        run_command "$HOME/app/.venv/bin/streamlit run $app_path" "start streamlit application"
        ;;
    "mcp_server")
        run_command "$HOME/app/.venv/bin/python $app_path" "start mcpserver application"
        ;;
    "nginx")
        daemon_path="${DOWNLOAD_DIR}/nginx.conf"
        if [ ! -f "$daemon_path" ]; then
            echo "ERROR: nginx configuration file $daemon_path does not exist!" >&2
            exit 1
        fi
        run_command "nginx -c $daemon_path -g 'daemon off;'" "start nginx service"
        ;;
    *)
        echo "ERROR: Unsupported SDK type: $SDK, only gradio/streamlit/mcp_server/nginx are supported!" >&2
        exit 1
        ;;
esac

echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S')] 🎉 all operations done! application started."