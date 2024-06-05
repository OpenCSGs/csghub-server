from pycsghub.snapshot_download import snapshot_download
import os

DOWNLOAD_DIR = "/data"
REPO_ID = os.environ['REPO_ID']
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']
ENDPOINT = os.environ['HF_ENDPOINT']
os.environ['CSGHUB_DOMAIN'] = ENDPOINT
snapshot_download(REPO_ID, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT, token=TOKEN)
