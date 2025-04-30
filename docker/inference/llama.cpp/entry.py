from pycsghub.snapshot_download import snapshot_download
import os

if __name__ == "__main__":
    model_file = os.environ['GGUF_ENTRY_POINT']
    model_file = model_file.replace("00001-of", "*")
    DOWNLOAD_DIR = "/workspace"
    REPO_ID = os.environ['REPO_ID']
    REVISION = os.getenv('REVISION', 'main')
    TOKEN = os.environ['ACCESS_TOKEN']
    ENDPOINT = os.environ['HF_ENDPOINT']
    os.environ['CSGHUB_DOMAIN'] = ENDPOINT
    snapshot_download(REPO_ID, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT,
                      revision=REVISION, token=TOKEN, allow_patterns=model_file)
