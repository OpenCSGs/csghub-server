#!/bin/bash
set -euo pipefail

: "${CLAW_EVAL_COMMAND:=batch}"
: "${CLAW_EVAL_CONFIG:=config_csghub.yaml}"
: "${CLAW_EVAL_TASKS:=normal}"
: "${CLAW_EVAL_TRACE_DIR:=traces}"

export CLAW_EVAL_EMIT_RESULTS=1

echo "Starting claw-eval ${CLAW_EVAL_COMMAND} for model ${CLAW_EVAL_MODEL:-unknown}..."
claw-eval-docker "${CLAW_EVAL_COMMAND}"

trace_root="/app/${CLAW_EVAL_TRACE_DIR}"
mkdir -p "${trace_root}"

results_file=$(find "${trace_root}" -name "batch_results.json" -type f -exec stat -c '%Y %n' {} + 2>/dev/null \
    | sort -n | tail -1 | cut -d' ' -f2-)
if [ -z "${results_file}" ] || [ ! -f "${results_file}" ]; then
    echo "No batch_results.json found under ${trace_root}"
    exit 1
fi

trace_dir=$(dirname "${results_file}")
summary_file="${trace_dir}/batch_summary.json"
if [ ! -f "${summary_file}" ]; then
    echo "No batch_summary.json found in ${trace_dir}"
    exit 1
fi

echo "Uploading claw-eval results from ${trace_dir}..."
python /etc/csghub/upload_files.py upload "${summary_file},${results_file}"
output=$(cat /tmp/output.txt)
echo "Claw evaluation output: ${output}"
echo "finish claw evaluation for ${CLAW_EVAL_MODEL:-unknown}"
