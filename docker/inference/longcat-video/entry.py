"""Download LongCat base and Avatar checkpoints from CSGHub."""

import os
import time
from pathlib import Path

from pycsghub.snapshot_download import snapshot_download
from requests.exceptions import ConnectionError, HTTPError


def download_model(
    repo_id: str,
    revision: str,
    local_dir: Path,
    allow_patterns: list[str] | None = None,
) -> None:
    endpoint = os.environ["HF_ENDPOINT"]
    token = os.environ.get("ACCESS_TOKEN")
    attempts = int(os.getenv("DOWNLOAD_RETRIES", "15"))
    local_dir.parent.mkdir(parents=True, exist_ok=True)
    for attempt in range(1, attempts + 1):
        try:
            snapshot_download(
                repo_id,
                cache_dir="/workspace/.cache",
                local_dir=local_dir,
                endpoint=endpoint,
                token=token,
                revision=revision,
                allow_patterns=allow_patterns,
            )
            return
        except (ConnectionError, HTTPError):
            if attempt == attempts:
                raise
            time.sleep(10)


def resolve_model_type(repo_id: str) -> str:
    model_name = Path(repo_id).name
    if model_name == "LongCat-Video-Avatar":
        return "avatar-v1.0"
    if model_name == "LongCat-Video-Avatar-1.5":
        return "avatar-v1.5"
    raise ValueError(f"unsupported LongCat Avatar repository: {repo_id}")


def base_allow_patterns(model_type: str) -> list[str] | None:
    if os.getenv("LONGCAT_DOWNLOAD_FULL") == "1":
        return None

    patterns = [
        "tokenizer/*",
        "text_encoder/*",
        "vae/*",
    ]
    if model_type == "avatar-v1.0":
        patterns.append("scheduler/*")
    return patterns


def avatar_allow_patterns(model_type: str) -> list[str] | None:
    if os.getenv("LONGCAT_DOWNLOAD_FULL") == "1":
        return None

    if model_type == "avatar-v1.5":
        return [
            "config.json",
            "model_index.json",
            "scheduler/*",
            "base_model_int8/*",
            "lora/dmd_lora.safetensors",
            "whisper-large-v3/*",
            "vocal_separator/*",
        ]
    return [
        "config.json",
        "model_index.json",
        "avatar_single/*",
        "avatar_multi/*",
        "chinese-wav2vec2-base/*",
        "vocal_separator/*",
    ]


def main() -> None:
    os.environ["CSGHUB_DOMAIN"] = os.environ["HF_ENDPOINT"]
    workspace = Path(os.getenv("WORKSPACE", "/workspace"))
    repo_id = os.environ["REPO_ID"]
    model_type = resolve_model_type(repo_id)
    namespace, separator, _ = repo_id.rpartition("/")
    inferred_base_model_id = f"{namespace}/LongCat-Video" if separator else "LongCat-Video"
    base_model_id = os.getenv("BASE_MODEL_ID", inferred_base_model_id)
    download_model(
        base_model_id,
        os.getenv("BASE_MODEL_REVISION", "main"),
        workspace / base_model_id,
        allow_patterns=base_allow_patterns(model_type),
    )
    download_model(
        repo_id,
        os.getenv("REVISION", "main"),
        workspace / repo_id,
        allow_patterns=avatar_allow_patterns(model_type),
    )


if __name__ == "__main__":
    main()
