#!/bin/sh

set -euo pipefail

readonly SDK_GRADIO="gradio"
readonly SDK_STREAMLIT="streamlit"
readonly SDK_DOCKER="docker"
readonly SDK_MCPSERVER="mcp_server"
readonly SDK_NGINX="nginx"
readonly LFS_FOLDER_NAME='.lfs.opencsg.co'
readonly APP_FILE="app.py"

readonly START_TIME=$(date +%s)

log() {
    local level="$1"
    local message="$2"
    local current_time=$(date +%s)
    local elapsed=$((current_time - START_TIME))
    local timestamp=$(printf "%04d" $elapsed)
    # Use tr to convert level to uppercase for better compatibility
    local level_uppercase=$(echo "$level" | tr '[:lower:]' '[:upper:]')
    echo "${level_uppercase}[${timestamp}] ${message}"
}

init_workdir() {
    CURRENT_DIR=$(cd "$(dirname "$0")" && pwd 2>/dev/null)
    source_dir="${CURRENT_DIR}"
    log "INFO" "Initialized work directory: ${CURRENT_DIR}"
}

parse_arguments() {
    if [ $# -lt 9 ]; then
        log "ERROR" "Incorrect number of arguments"
        log "INFO" "Usage: $0 <datafolder> <reponame> <username> <gittoken> <fullrepourl> <gitref> <sdk> <python_version> <device> <driver_version>"
        exit 1
    fi

    datafolder="$1"
    reponame="$2"
    username="$3"
    gittoken="$4"
    fullrepourl="$5"
    gitref="$6"
    sdk="$7"
    python_version="$8"
    device="$9"
    driver_version="${10:-}"

    if [ "$device" != "cpu" ] && [ "$device" != "gpu" ]; then
        log "ERROR" "Device type must be 'cpu' or 'gpu', current: $device"
        exit 1
    fi
    log "INFO" "Parsed arguments: datafolder=${datafolder}, reponame=${reponame}, sdk=${sdk}, device=${device}"
}

process_repo_url() {
    case "$fullrepourl" in
        https://*)
            modified_repourl=$(echo "$fullrepourl" | sed 's/https:\/\///')
            space_respository="https://${username}:${gittoken}@${modified_repourl}"
            ;;
        http://*)
            modified_repourl=$(echo "$fullrepourl" | sed 's/http:\/\///')
            space_respository="http://${username}:${gittoken}@${modified_repourl}"
            ;;
        *)
            log "ERROR" "Invalid Git URL format: $fullrepourl (must be http/https)"
            exit 1
            ;;
    esac

    repo="${datafolder}/${reponame}"
}

clone_repository() {
    log "INFO" "Starting repository clone: ${reponame} -> ${repo}"
    
    if [ -d "$repo" ]; then
        log "INFO" "Removing existing directory: ${repo}"
        rm -rf "$repo" || {
            log "ERROR" "Failed to remove existing directory: ${repo}"
            exit 1
        }
    fi

    # log "INFO" "Configured Git LFS (GIT_LFS_SKIP_SMUDGE=1)"
    # export GIT_LFS_SKIP_SMUDGE=1

    if ! git clone "$space_respository" "$repo"; then
        log "ERROR" "Failed to clone repository (check URL/credentials)"
        exit 1
    fi

    cd "$repo" || {
        log "ERROR" "Cannot enter repository directory: ${repo}"
        exit 1
    }
    if ! git checkout "$gitref"; then
        log "ERROR" "Failed to checkout Git ref: ${gitref} (does it exist?)"
        exit 1
    fi

    log "INFO" "Pulling LFS files (this may take time for large files)..."
    if ! git lfs pull; then
        log "ERROR" "Failed to pull LFS files. Ensure Git LFS is installed and repository has valid LFS tracking."
        exit 1
    fi

    log "INFO" "Successfully cloned repository and checked out ref: ${gitref}"
}


generate_dockerfile() {
    log "INFO" "Generating Dockerfile for SDK: ${sdk}"
    
    if [ "$sdk" = "$SDK_DOCKER" ]; then
        if [ ! -f "${repo}/Dockerfile" ]; then
            log "ERROR" "Dockerfile not found at: ${repo}/Dockerfile"
            exit 1
        fi
        log "INFO" "Using existing Dockerfile (SDK_DOCKER)"
        return 0
    fi

    local file="Dockerfile-python${python_version}"
    
    if [ "$device" = "gpu" ]; then
        if [ -z "$driver_version" ]; then
            driver_version="11.8.0"
            log "INFO" "Using default CUDA driver version: ${driver_version}"
        fi
        file="${file}-cuda${driver_version}"
    fi

    if [ "$sdk" = "$SDK_NGINX" ]; then
        if [ ! -f "${repo}/nginx.conf" ]; then
            log "ERROR" "nginx.conf not found at: ${repo}/nginx.conf"
            exit 1
        fi
        file="Dockerfile-nginx"
    fi

    local sourcefile="${source_dir}/${file}"
    log "INFO" "Checking Dockerfile source: ${sourcefile}"
    if [ ! -f "$sourcefile" ]; then
        log "ERROR" "Dockerfile source not found: ${sourcefile}"
        exit 1
    fi
    if ! cp "$sourcefile" "${repo}/Dockerfile"; then
        log "ERROR" "Failed to copy Dockerfile to: ${repo}/Dockerfile"
        exit 1
    fi
    log "INFO" "Successfully generated Dockerfile: ${repo}/Dockerfile"
}

generate_start_script() {
    log "INFO" "Generating start script for SDK: ${sdk}"
    local script_path="${repo}/start_entrypoint.sh"
    
    case "$sdk" in
        "$SDK_GRADIO" | "$SDK_MCPSERVER")
            cat > "$script_path" <<EOF
#!/bin/sh
python3 ${APP_FILE}
EOF
            ;;
        "$SDK_STREAMLIT")
            cat > "$script_path" <<EOF
#!/bin/sh
streamlit run ${APP_FILE}
EOF
            ;;
        "$SDK_DOCKER" | "$SDK_NGINX")
            log "INFO" "No start script needed for SDK: ${sdk}"
            return 0
            ;;
        *)
            log "ERROR" "Unsupported SDK type: ${sdk}"
            exit 1
            ;;
    esac

    chmod 755 "$script_path" || {
        log "ERROR" "Failed to set permissions for: ${script_path}"
        exit 1
    }
    log "INFO" "Generated start script: ${script_path}"
}

generate_dependency_files() {
    log "INFO" "Checking dependency files for SDK: ${sdk}"
    
    if [ "$sdk" != "$SDK_GRADIO" ] && [ "$sdk" != "$SDK_STREAMLIT" ] && [ "$sdk" != "$SDK_MCPSERVER" ]; then
        log "INFO" "No dependency files needed for SDK: ${sdk}"
        return 0
    fi

    local requirementsfile="${repo}/requirements.txt"
    [ -f "$requirementsfile" ] || touch "$requirementsfile" || {
        log "ERROR" "Failed to create requirements.txt: ${requirementsfile}"
        exit 1
    }
    
    local packagefile="${repo}/packages.txt"
    [ -f "$packagefile" ] || touch "$packagefile" || {
        log "ERROR" "Failed to create packages.txt: ${packagefile}"
        exit 1
    }
    
    local prerequirementsfile="${repo}/pre-requirements.txt"
    [ -f "$prerequirementsfile" ] || touch "$prerequirementsfile" || {
        log "ERROR" "Failed to create pre-requirements.txt: ${prerequirementsfile}"
        exit 1
    }
    log "INFO" "Ensured dependency files exist (requirements.txt, packages.txt, pre-requirements.txt)"
}

finalize() {
    log "INFO" "Finalizing repository setup"
    
    echo "$space_respository" > "${repo}/SPACE_REPOSITORY" || {
        log "ERROR" "Failed to save repository URL to: ${repo}/SPACE_REPOSITORY"
        exit 1
    }
    
    rm -rf "${repo}/.git" || {
        log "ERROR" "Failed to clean Git directory: ${repo}/.git"
        exit 1
    }
    
    log "INFO" "All operations completed successfully"
}

generate_dockerignore() {
    log "INFO" "Generating .dockerignore file"
    local dockerignore_path="${repo}/.dockerignore"

    if [ -f "$dockerignore_path" ]; then
        local backup="${dockerignore_path}.$(date +%s).bak"
        mv "$dockerignore_path" "$backup"
        log "INFO" "Existing .dockerignore backed up to: ${backup}"
    fi

    cat > "$dockerignore_path" << 'EOF'
# Git
.git
.gitignore
.gitmodules

.lfs
*.lfs.*

*.log
*.tmp
*.swp
.DS_Store
Thumbs.db

# Python
__pycache__
*.pyc
*.pyo
*.pyd
.pytest_cache
.coverage

# Virtual environments
venv/
env/
.venv/
ENV/

# Dependency management
pip-selfcheck.json
poetry.lock
*.egg-info/

# Node.js (unless using Streamlit components)
node_modules/
package-lock.json

# IDE and editors
.idea/
.vscode/
*.sublime-project
*.sublime-workspace

# Sensitive files
secrets.txt
config.local.json
.env.local
*.pem
*.key

# Tests
tests/
test/
__tests__/
EOF

    if [ $? -eq 0 ]; then
        log "INFO" "Successfully generated: ${dockerignore_path}"
    else
        log "ERROR" "Failed to write .dockerignore file"
        exit 1
    fi
}

main() {
    init_workdir
    parse_arguments "$@"
    process_repo_url
    clone_repository
    generate_dockerfile
    generate_start_script
    generate_dependency_files
    generate_dockerignore
    finalize
}

main "$@"