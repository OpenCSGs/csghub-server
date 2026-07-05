import argparse
import json
import os
from datetime import datetime
from pathlib import Path

import oss2
from minio import Minio

access_key_id = os.environ["S3_ACCESS_ID"]
access_key_secret = os.environ["S3_ACCESS_SECRET"]
bucket_name = os.environ["S3_BUCKET"]
endpoint = os.environ["S3_ENDPOINT"]
s3_ssl_enabled = json.loads(os.environ["S3_SSL_ENABLED"])

if endpoint.find("aliyuncs.com") == -1:
    client = Minio(endpoint, access_key=access_key_id, secret_key=access_key_secret, secure=s3_ssl_enabled)
else:
    auth = oss2.Auth(access_key_id, access_key_secret)
    bucket = oss2.Bucket(auth, endpoint, bucket_name)


def generate_file_name(name: str) -> str:
    now = datetime.now()
    return f"{name}_{now.strftime('%Y%m%d_%H%M%S')}"


def upload_to_minio(object_name: str, location_file: str) -> None:
    found = client.bucket_exists(bucket_name)
    if not found:
        client.make_bucket(bucket_name)
    client.fput_object(bucket_name, object_name, location_file)


def upload_to_ali(object_name: str, location_file: str) -> None:
    bucket.put_object_from_file(object_name, location_file)


def upload(files: str) -> None:
    output = []
    schema = "https" if s3_ssl_enabled else "http"
    base_name = generate_file_name("claw_eval_result")
    for file in files.split(","):
        file = file.strip()
        if not file:
            continue
        path = Path(file)
        suffix = path.suffix or ".json"
        stem = path.stem or "result"
        object_name = f"evaluation/{base_name}_{stem}{suffix}"
        if endpoint.find("aliyuncs.com") != -1:
            upload_to_ali(object_name, file)
            file_url = f"https://{bucket_name}.{endpoint}/{object_name}"
        else:
            upload_to_minio(object_name, file)
            file_url = f"{schema}://{endpoint}/{bucket_name}/{object_name}"
        output.append(file_url)
    with open("/tmp/output.txt", "w", encoding="utf-8") as out_file:
        out_file.write(",".join(output))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Upload claw-eval result files.")
    subparsers = parser.add_subparsers(dest="command", required=True)
    upload_parser = subparsers.add_parser("upload", help="upload files")
    upload_parser.add_argument("files", type=str, help="comma separated file paths")
    args = parser.parse_args()
    if args.command == "upload":
        upload(args.files)
