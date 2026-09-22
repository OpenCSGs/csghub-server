"""Exercise each framework's `summary` entry point the way start.sh calls it.

Earlier changes were checked by parsing the scripts, which does not reach the
argument wiring, so runtime faults survived review. These tests invoke the real
command line instead. lm-evaluation-harness used to be a third
framework here and has since been removed from the repository.

Run: python3 docker/evaluation/tests/summary_cli_test.py
"""
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
SHA_A = "1111111111111111111111111111111111111111"
SHA_B = "2222222222222222222222222222222222222222"

ENV = dict(
    os.environ,
    S3_ACCESS_ID="dummy", S3_ACCESS_SECRET="dummy", S3_BUCKET="dummy",
    S3_ENDPOINT="127.0.0.1:9000", S3_SSL_ENABLED="false",
)


def run_summary(script, workspace, args):
    """Invoke `summary` exactly as start.sh does, with /workspace redirected."""
    src = open(os.path.join(ROOT, script), encoding="utf-8").read()
    src = src.replace("/workspace/output/final/", os.path.join(workspace, "final") + "/")
    patched = os.path.join(workspace, "upload_files.py")
    open(patched, "w", encoding="utf-8").write(src)
    os.makedirs(os.path.join(workspace, "final"), exist_ok=True)
    proc = subprocess.run([sys.executable, patched, "summary"] + args,
                          capture_output=True, text=True, env=ENV)
    if proc.returncode != 0:
        raise AssertionError(f"{script} summary failed:\n{proc.stdout[-2000:]}\n{proc.stderr[-2000:]}")
    with open(os.path.join(workspace, "final", "upload.json"), encoding="utf-8") as f:
        return json.load(f)


def evalscope_report(score):
    return {"model_name": "model", "dataset_name": "arc", "metrics": [{
        "identity": {"name": "accuracy", "aggregation": "mean"}, "legacy_name": "accuracy",
        "score": score, "categories": [{"subsets": [{"name": "ARC-Easy", "score": score}]}]}]}


def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    json.dump(obj, open(path, "w", encoding="utf-8"))
    return path


def check_evalscope(ws):
    a = write(f"{ws}/outputs/run0/ts/reports/model/arc.json", evalscope_report(0.5))
    b = write(f"{ws}/outputs/run1/ts/reports/model/arc.json", evalscope_report(0.7))
    out = run_summary("evalscope/upload_files.py", ws, [
        "--file", a, b, "--tasks", "arc",
        "--run-map", f"{ws}/outputs/run0=org-a/model@{SHA_A},{ws}/outputs/run1=org-b/model@{SHA_B}"])
    rows = out["summary"]["data"]
    assert [r["repo_id"] for r in rows] == ["org-a/model", "org-b/model"], rows
    assert [r["revision"] for r in rows] == [SHA_A, SHA_B], rows
    assert [r["score"] for r in rows] == [0.5, 0.7], rows
    assert any(c["key"] == "repo_id" for c in out["summary"]["column"])
    print("  OK evalscope: two runs attributed by run directory")


def check_opencompass(ws):
    row = {"dataset": "arc", "metric": "accuracy", "mode": "gen", "model": 0.5}
    a = write(f"{ws}/output/run0/ts/summary/s.json", [row])
    b = write(f"{ws}/output/run1/ts/summary/s.json", [dict(row, model=0.7)])
    out = run_summary("opencompass/upload_files.py", ws, [
        "--file", a, b, "--tasks", "arc",
        "--run-map", f"{ws}/output/run0=org-a/model@{SHA_A},{ws}/output/run1=org-b/model@{SHA_B}"])
    rows = out["summary"]["data"]
    assert [r["repo_id"] for r in rows] == ["org-a/model", "org-b/model"], rows
    assert [r["revision"] for r in rows] == [SHA_A, SHA_B], rows
    print("  OK opencompass: two runs attributed by run directory")


if __name__ == "__main__":
    failures = []
    for name, check in (("evalscope", check_evalscope),
                        ("opencompass", check_opencompass)):
        with tempfile.TemporaryDirectory() as ws:
            try:
                check(ws)
            except Exception as exc:  # noqa: BLE001 - report every framework
                failures.append(f"{name}: {exc}")
                print(f"  FAIL {name}: {exc}")
    if failures:
        sys.exit(1)
    print("\nall summary entry points OK")
