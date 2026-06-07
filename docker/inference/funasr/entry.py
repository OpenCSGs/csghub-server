from pathlib import Path
import os
import time

from pycsghub.snapshot_download import snapshot_download
from requests.exceptions import ConnectionError, HTTPError


DOWNLOAD_DIR = "/workspace"
REPO_ID = os.environ["REPO_ID"]
REVISION = os.getenv("REVISION", "main")
TOKEN = os.environ["ACCESS_TOKEN"]
ENDPOINT = os.environ["HF_ENDPOINT"]
os.environ["CSGHUB_DOMAIN"] = ENDPOINT

max_retries = 15
retry_count = 0
local_dir = Path(DOWNLOAD_DIR) / REPO_ID

while retry_count < max_retries:
    try:
        snapshot_download(
            REPO_ID,
            cache_dir=DOWNLOAD_DIR,
            local_dir=local_dir,
            endpoint=ENDPOINT,
            token=TOKEN,
            revision=REVISION,
        )
        break
    except (ConnectionError, HTTPError) as e:
        retry_count += 1
        print(f"exception occurred: {e}. Retrying in 10 seconds... (Attempt {retry_count}/{max_retries})")
        time.sleep(10)

if not local_dir.exists():
    raise RuntimeError(f"model download failed: {REPO_ID} was not downloaded to {local_dir}")
