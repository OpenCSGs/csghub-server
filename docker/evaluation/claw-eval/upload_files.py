import argparse
import json
import os
from datetime import datetime
from pathlib import Path

access_key_id = os.environ.get("S3_ACCESS_ID", "")
access_key_secret = os.environ.get("S3_ACCESS_SECRET", "")
bucket_name = os.environ["S3_BUCKET"]
endpoint = os.environ["S3_ENDPOINT"]
s3_ssl_enabled = json.loads(os.environ.get("S3_SSL_ENABLED", "false"))

# Gateway upload configuration
upload_via_gateway = os.environ.get("UPLOAD_VIA_GATEWAY", "false").lower() == "true"
gateway_url = os.environ.get("STARHUB_SERVER_PUBLIC_DOMAIN", "").rstrip("/")
access_token = os.environ.get("ACCESS_TOKEN", "")

if not upload_via_gateway:
    import oss2
    from minio import Minio
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


def upload_via_gateway_func(object_name: str, location_file: str) -> str:
    if not gateway_url:
        raise ValueError("STARHUB_SERVER_PUBLIC_DOMAIN is not set but UPLOAD_VIA_GATEWAY=true")
    if not access_token:
        raise ValueError("ACCESS_TOKEN is not set but UPLOAD_VIA_GATEWAY=true")
    import requests
    url = f"{gateway_url}/api/v1/storage/{bucket_name}/{object_name}"
    with open(location_file, "rb") as f:
        resp = requests.put(
            url, data=f,
            headers={"Authorization": f"Bearer {access_token}", "Content-Type": "application/octet-stream"},
            timeout=300,
        )
        resp.raise_for_status()
    print(f"Uploaded {location_file} to gateway: {url}")
    return url


def upload(files: str) -> None:
    output = []
    if upload_via_gateway:
        upload_fn = upload_via_gateway_func
        print("Using storageGateway proxy mode for upload")
    else:
        schema = "https" if s3_ssl_enabled else "http"
        def upload_fn(object_name, location_file):
            if endpoint.find("aliyuncs.com") != -1:
                upload_to_ali(object_name, location_file)
                return f"https://{bucket_name}.{endpoint}/{object_name}"
            else:
                upload_to_minio(object_name, location_file)
                return f"{schema}://{endpoint}/{bucket_name}/{object_name}"
        print("Using direct S3/OSS upload")

    base_name = generate_file_name("claw_eval_result")
    for file in files.split(","):
        file = file.strip()
        if not file:
            continue
        path = Path(file)
        suffix = path.suffix or ".json"
        stem = path.stem or "result"
        object_name = f"evaluation/{base_name}_{stem}{suffix}"
        file_url = upload_fn(object_name, file)
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
