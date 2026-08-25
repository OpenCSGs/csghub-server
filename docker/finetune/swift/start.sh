#!/usr/bin/env bash

set -u

REPO_ID="${REPO_ID:-Qwen/Qwen2.5-7B-Instruct}"
REVISION="${REVISION:-main}"
CONTEXT_PATH="${CONTEXT_PATH:-}"
MODEL_NAME="${REPO_ID##*/}"
MODEL_NAME="${MODEL_NAME%%:*}"

touch /workspace/.csghub_init

swift_path="$(python -c 'import pathlib, swift; print(pathlib.Path(swift.__file__).resolve().parent)')"
if [ ! -d "$swift_path" ]; then
    echo "Unable to locate the ms-swift package directory: $swift_path" >&2
    exit 1
fi

model_template="$(python /etc/csghub/get_model_info_clean.py "$MODEL_NAME")"
echo "$model_template"
IFS=',' read -r model_type template_type _ <<< "$model_template"

export REPO_ID REVISION model_type template_type swift_path
python - <<'PY'
import os
import re
from pathlib import Path

swift_path = Path(os.environ["swift_path"])
repo_id = os.environ["REPO_ID"]
model_type = os.environ["model_type"]
template_type = os.environ["template_type"]

ui_paths = [
    swift_path / "ui/llm_train/model.py",
    swift_path / "ui/llm_infer/model.py",
    swift_path / "ui/llm_export/model.py",
    swift_path / "ui/llm_eval/model.py",
]

for ui_path in ui_paths:
    if not ui_path.is_file():
        continue
    content = ui_path.read_text()
    content = content.replace("Qwen/Qwen2.5-7B-Instruct", repo_id)
    if model_type:
        content = re.sub(
            r"elem_id='model_type'(?:,\s*value='[^']*')?",
            f"elem_id='model_type', value='{model_type}'",
            content,
        )
    if template_type:
        content = re.sub(
            r"elem_id='template'(?:,\s*value='[^']*')?",
            f"elem_id='template', value='{template_type}'",
            content,
        )
    ui_path.write_text(content)

infer_ui_path = swift_path / "ui/llm_infer/llm_infer.py"
if infer_ui_path.is_file():
    content = infer_ui_path.read_text()
    content = content.replace(
        "gr.Textbox(elem_id='port', lines=1, value='8000', scale=4)",
        "gr.Textbox(elem_id='port', lines=1, value='9000', scale=4)",
    )
    infer_ui_path.write_text(content)

hub_utils_path = swift_path / "utils/hub_utils.py"
if hub_utils_path.is_file():
    content = hub_utils_path.read_text()
    content = content.replace(
        "hub.download_model(model_id_or_path, revision, ignore_patterns",
        "hub.download_model(model_id_or_path, revision or os.getenv('REVISION'), ignore_patterns",
    )
    hub_utils_path.write_text(content)
PY

export GRADIO_ROOT_PATH="${CONTEXT_PATH}/proxy/7860"
export USE_CSGHUB_TRANSFER=1
export USE_HF=1
export SWIFT_UI_LANG=en

ascend_env=/usr/local/Ascend/ascend-toolkit/set_env.sh
if [ -f "$ascend_env" ]; then
    # shellcheck disable=SC1090
    source "$ascend_env"
fi

exec swift web-ui

