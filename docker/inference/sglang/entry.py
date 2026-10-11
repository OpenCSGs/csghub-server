from pycsghub.snapshot_download import snapshot_download
import os
import sys
import time
from requests.exceptions import ConnectionError,HTTPError

DOWNLOAD_DIR = "/workspace"
REPO_ID = os.environ['REPO_ID']
REVISION = os.getenv('REVISION', 'main')
SPEC_DRAFT_REPO_ID = os.getenv('SPEC_DRAFT_REPO_ID', '')
DRAFT_REVISION = os.getenv('DRAFT_REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']
ENDPOINT = os.environ['HF_ENDPOINT']
os.environ['CSGHUB_DOMAIN'] = ENDPOINT
max_retries = 15
ignore_patterns = ["*.bin"]

def download(repo_id, revision, ignore=None):
    retry_count = 0
    while retry_count < max_retries:
        try:
            snapshot_download(repo_id, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT, token=TOKEN, revision=revision, ignore_patterns=ignore)
            break
        except (ConnectionError, HTTPError) as e:
            retry_count += 1
            print(f"exception occurred: {e}. Retrying in 10 seconds... (Attempt {retry_count}/{max_retries})")
            time.sleep(10)
    else:
        # The hub may be briefly unreachable while the node still holds a
        # complete cached copy (the engine runs with HF_HUB_OFFLINE=1). The
        # SDK's local_files_only probe only checks that at least one file was
        # ever cached (no completeness, no revision), so verify the .mv marker
        # instead: save_model_version() writes it only after a full snapshot
        # download finishes and records the downloaded revision.
        version_file = os.path.join(DOWNLOAD_DIR, repo_id, '.mv')
        if not os.path.exists(version_file):
            sys.exit(f"failed to download {repo_id}@{revision} after {max_retries} attempts and no complete local cache exists")
        if revision:
            with open(version_file, 'r') as f:
                if f.read().strip() != f"Revision:{revision}":
                    sys.exit(f"cached {repo_id} revision does not match requested {revision} and hub is unreachable; refusing to start from a stale cache")
        print(f"hub unreachable for {repo_id}@{revision}; starting from existing local cache")

download(REPO_ID, REVISION, ignore_patterns)
if SPEC_DRAFT_REPO_ID:
    download(SPEC_DRAFT_REPO_ID, DRAFT_REVISION)
