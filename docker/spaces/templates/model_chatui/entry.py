from pycsghub.snapshot_download import snapshot_download
import os

DOWNLOAD_DIR = "/workspace"
REPO_ID = os.getenv('MODEL_NAME','')
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.getenv('ACCESS_TOKEN','')
ENDPOINT = os.getenv('HF_ENDPOINT','https://hub.opencsg.com')
REPO_TYPE = "model"
os.environ['CSGHUB_DOMAIN'] = ENDPOINT
snapshot_download(REPO_ID, repo_type=REPO_TYPE, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT, token=TOKEN)
