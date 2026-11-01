from huggingface_hub import snapshot_download
import os

DOWNLOAD_DIR = "/workspace"
REPO_ID = os.environ['REPO_ID']
REPO_NAME = REPO_ID.split('/')[1]
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']
ENDPOINT = os.environ['HF_ENDPOINT'] + "/hf"
snapshot_download(repo_id=REPO_ID, cache_dir=DOWNLOAD_DIR, repo_type="model",
                  endpoint=ENDPOINT, token=TOKEN, revision=REVISION, local_dir="/workspace/"+REPO_ID+"-ori")
