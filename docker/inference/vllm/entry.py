from huggingface_hub import snapshot_download
import os
import subprocess


DOWNLOAD_DIR = "/data"
REPO_ID = os.environ['REPO_ID']
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']


def parse_and_download():

    local_dir = f'{DOWNLOAD_DIR}/{REPO_ID}'
    snapshot_download(repo_id=REPO_ID, revision=REVISION, token=TOKEN, local_dir=local_dir, repo_type="model")

    other_args = ['--model', local_dir]

    return other_args


def run_app():
    parse_and_download()


if __name__ == "__main__":
    run_app()
