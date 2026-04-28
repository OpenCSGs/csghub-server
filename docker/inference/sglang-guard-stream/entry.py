from pycsghub.snapshot_download import snapshot_download
import os
import time
from requests.exceptions import ConnectionError,HTTPError

DOWNLOAD_DIR = "/workspace"
REPO_ID = os.environ['REPO_ID']
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']
ENDPOINT = os.environ['HF_ENDPOINT']
os.environ['CSGHUB_DOMAIN'] = ENDPOINT
max_retries = 15
retry_count = 0
ignore_patterns = ["*.bin"]
while retry_count < max_retries:
    try:
        snapshot_download(REPO_ID, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT, token=TOKEN, revision=REVISION,ignore_patterns=ignore_patterns)
        break
    except (ConnectionError, HTTPError) as e:
        retry_count += 1
        print(f"exception occurred: {e}. Retrying in 10 seconds... (Attempt {retry_count}/{max_retries})")
        time.sleep(10)
