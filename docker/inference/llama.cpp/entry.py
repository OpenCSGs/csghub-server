import fnmatch
import os
import time
import urllib.parse
from pathlib import PurePosixPath

import requests
from requests.exceptions import ConnectionError, HTTPError

DOWNLOAD_DIR = "/workspace"
MAX_RETRIES = 15


def sibling_size(item):
    """Return the object size, preferring the LFS object size when present."""
    lfs = item.get("lfs") or {}
    lfs_size = lfs.get("size") if isinstance(lfs, dict) else 0
    try:
        size = int(item.get("size") or 0)
        lfs_size = int(lfs_size or 0)
    except (TypeError, ValueError):
        return 0
    return max(size, lfs_size)


def is_relative_repo_file(name):
    if not name or os.path.isabs(name):
        return False
    return ".." not in PurePosixPath(name).parts


def files_for_pattern(siblings, pattern):
    """Map pattern-matched repo paths to remote sizes.

    An empty result means the local copy must not be trusted: either nothing
    matched, or a matched file has no usable size.
    """
    matched = {}
    for item in siblings:
        name = item.get("rfilename")
        if not isinstance(name, str) or not is_relative_repo_file(name):
            continue
        if not fnmatch.fnmatch(name, pattern):
            continue
        size = sibling_size(item)
        if size <= 0:
            return {}
        matched[name] = size
    return matched


def local_files_match(repo_dir, expected):
    if not expected:
        return False
    for name, size in expected.items():
        path = os.path.join(repo_dir, name)
        try:
            if os.path.getsize(path) != size:
                return False
        except OSError:
            return False
    return True


def fetch_siblings(endpoint, repo_id, revision, token, timeout=30):
    quoted_revision = urllib.parse.quote(revision, safe="")
    url = f"{endpoint.rstrip('/')}/csg/api/models/{repo_id}/revision/{quoted_revision}"
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    response = requests.get(url, headers=headers, params={"blobs": "true"}, timeout=timeout)
    response.raise_for_status()
    siblings = response.json().get("siblings") or []
    if not isinstance(siblings, list):
        return []
    return siblings


def should_skip_download(repo_dir, pattern, siblings):
    return local_files_match(repo_dir, files_for_pattern(siblings, pattern))


def download_snapshot(repo_id, revision, token, endpoint, allow_patterns):
    from pycsghub.snapshot_download import snapshot_download

    retry_count = 0
    while retry_count < MAX_RETRIES:
        try:
            snapshot_download(
                repo_id,
                cache_dir=DOWNLOAD_DIR,
                endpoint=endpoint,
                revision=revision,
                token=token,
                allow_patterns=allow_patterns,
            )
            return
        except (ConnectionError, HTTPError) as exc:
            retry_count += 1
            print(f"exception occurred: {exc}. Retrying in 10 seconds... (Attempt {retry_count}/{MAX_RETRIES})")
            time.sleep(10)


def main():
    model_file = os.environ["GGUF_ENTRY_POINT"].replace("00001-of", "*")
    repo_id = os.environ["REPO_ID"]
    revision = os.getenv("REVISION", "main")
    token = os.environ["ACCESS_TOKEN"]
    endpoint = os.environ["HF_ENDPOINT"]
    os.environ["CSGHUB_DOMAIN"] = endpoint
    repo_dir = os.path.join(DOWNLOAD_DIR, repo_id)

    try:
        siblings = fetch_siblings(endpoint, repo_id, revision, token)
        skip = should_skip_download(repo_dir, model_file, siblings)
    except Exception as exc:
        print(f"could not compare local file sizes, downloading: {exc}")
    else:
        if skip:
            print(f"skip download, local files under {repo_dir} match remote size for {model_file}")
            return
        print(f"local files under {repo_dir} do not match remote size for {model_file}, downloading")

    download_snapshot(repo_id, revision, token, endpoint, model_file)


if __name__ == "__main__":
    main()
