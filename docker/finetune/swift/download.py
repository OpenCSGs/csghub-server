import os
from pathlib import Path

from huggingface_hub import snapshot_download


def download(repo_id: str, repo_type: str, revision: str, local_dir: Path) -> None:
    local_dir.parent.mkdir(parents=True, exist_ok=True)
    snapshot_download(
        repo_id=repo_id,
        repo_type=repo_type,
        revision=revision or None,
        endpoint=os.environ.get("HF_ENDPOINT"),
        token=os.environ.get("HF_TOKEN"),
        local_dir=local_dir,
    )


if __name__ == "__main__":
    workspace = Path(os.environ.get("FINETUNE_WORK_DIR", "/workspace"))
    download(
        os.environ["MODEL_ID"],
        "model",
        os.environ.get("REVISION", ""),
        workspace / "models" / os.environ["MODEL_ID"],
    )
    download(
        os.environ["DATASET_ID"],
        "dataset",
        os.environ.get("DATASET_REVISION", ""),
        workspace / "datasets" / os.environ["DATASET_ID"],
    )
