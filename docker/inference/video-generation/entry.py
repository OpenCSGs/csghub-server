from pycsghub.snapshot_download import snapshot_download
import os
import time
from requests.exceptions import ConnectionError, HTTPError

DOWNLOAD_DIR = "/workspace"
REPO_ID = os.environ['REPO_ID']
REVISION = os.getenv('REVISION', 'main')
TOKEN = os.environ['ACCESS_TOKEN']
ENDPOINT = os.environ['HF_ENDPOINT']
HF_TASK = os.getenv('HF_TASK', '')
os.environ['CSGHUB_DOMAIN'] = ENDPOINT
max_retries = 15
retry_count = 0

# Download main repository
while retry_count < max_retries:
    try:
        snapshot_download(
            REPO_ID,
            cache_dir=DOWNLOAD_DIR,
            endpoint=ENDPOINT,
            token=TOKEN,
            revision=REVISION
        )
        break
    except (ConnectionError, HTTPError) as e:
        retry_count += 1
        print(f"exception occurred: {e}. Retrying in 10 seconds... (Attempt {retry_count}/{max_retries})")
        time.sleep(10)

# Handle loras directory and download lora files
repo_path = os.path.join(DOWNLOAD_DIR, REPO_ID)
loras_dir = os.path.join(repo_path, "loras")

# Ensure loras directory exists
if not os.path.exists(loras_dir):
    print(f"Creating loras directory: {loras_dir}")
    os.makedirs(loras_dir, exist_ok=True)

# Download lora files based on HF_TASK (always attempt download regardless of directory existence)
lora_repo_id = "AIWizards/Wan2.2-Distill-Loras"
lora_pattern = None

if HF_TASK == "text-to-video":
    lora_pattern = "wan2.2_t2v*.safetensors"
    print(f"Downloading text-to-video lora files: {lora_pattern}")
elif HF_TASK == "image-to-video":
    lora_pattern = "wan2.2_i2v*.safetensors"
    print(f"Downloading image-to-video lora files: {lora_pattern}")

if lora_pattern:
    retry_count = 0
    # Save the original CSGHUB_DOMAIN and override it for lora download
    original_csghub_domain = os.environ.get('CSGHUB_DOMAIN')
    os.environ['CSGHUB_DOMAIN'] = "https://hub.opencsg.com"
    while retry_count < max_retries:
        try:
            snapshot_download(
                lora_repo_id,
                cache_dir=DOWNLOAD_DIR,
                endpoint="https://hub.opencsg.com",
                allow_patterns=[lora_pattern]
            )

            # Move downloaded lora files to the loras directory
            lora_repo_path = os.path.join(DOWNLOAD_DIR, lora_repo_id)
            if os.path.exists(lora_repo_path):
                for root, dirs, files in os.walk(lora_repo_path):
                    for file in files:
                        if file.endswith('.safetensors') and (
                            (HF_TASK == "text-to-video" and file.startswith("wan2.2_t2v")) or
                            (HF_TASK == "image-to-video" and file.startswith("wan2.2_i2v"))
                        ):
                            src_file = os.path.join(root, file)
                            dst_file = os.path.join(loras_dir, file)
                            print(f"Copying {src_file} to {dst_file}")
                            os.rename(src_file, dst_file)

            print(f"Successfully downloaded lora files to {loras_dir}")
            break
        except (ConnectionError, HTTPError) as e:
            retry_count += 1
            print(
                f"exception occurred while downloading lora files: {e}. Retrying in 10 seconds... (Attempt {retry_count}/{max_retries})")
            time.sleep(10)
    # Restore the original CSGHUB_DOMAIN if it existed
    if original_csghub_domain:
        os.environ['CSGHUB_DOMAIN'] = original_csghub_domain
    elif 'CSGHUB_DOMAIN' in os.environ:
        del os.environ['CSGHUB_DOMAIN']
