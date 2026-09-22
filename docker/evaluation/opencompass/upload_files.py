import os
import argparse
import re
from datetime import datetime
from pathlib import Path
import pandas as pd
import json
import csv

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
    from minio import Minio
    from minio.error import S3Error
    import oss2
    if endpoint.find("aliyuncs.com") == -1:
        client = Minio(endpoint, access_key=access_key_id, secret_key=access_key_secret, secure=s3_ssl_enabled)
    else:
        auth = oss2.Auth(access_key_id, access_key_secret)
        bucket = oss2.Bucket(auth, endpoint, bucket_name)


def generate_file_name(name):
    # Get the current date and time
    now = datetime.now()

    # Format the string as YYYYMMDD_HHMMSS
    formatted_uuid = now.strftime("%Y%m%d_%H%M%S")

    return f"{name}_{formatted_uuid}"


def upload_to_minio(object_name, location_file):
    # Make the bucket if it doesn't exist.
    found = client.bucket_exists(bucket_name)
    if not found:
        client.make_bucket(bucket_name)
        print("Created bucket", bucket_name)
    else:
        print("Bucket", bucket_name, "already exists")

    # Upload the file, renaming it in the process
    client.fput_object(
        bucket_name, object_name, location_file,
    )


def upload_to_ali(object_name, location_file):
    # Upload the file, renaming it in the process
    bucket.put_object_from_file(object_name, location_file)


def upload_via_gateway_func(object_name, location_file):
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


def upload(files):
    output = []
    fileName = generate_file_name("result")
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

    for file in files.split(','):
        suffix = Path(file).suffix
        object_name = f"evaluation/{fileName}{suffix}"
        file_url = upload_fn(object_name, file)
        output.append(file_url)
    try:
        with open('/tmp/output.txt', 'w') as file:
            file.write(",".join(output))
            print("Output written to /tmp/output.txt")
        print(f'Successfully uploaded to {file_url}')
    except Exception as e:
        print(f"Error writing to file: {e}")


def csv_to_json(csv_file_path):
    # Read the CSV file
    with open(csv_file_path, mode='r', newline='', encoding='utf-8') as csv_file:
        csv_reader = csv.DictReader(csv_file)  # Using DictReader to read as dictionaries
        data = list(csv_reader)  # Convert to a list of dictionaries

    # Write the JSON output
    json_file_path = os.path.splitext(csv_file_path)[0] + '.json'
    with open(json_file_path, mode='w', encoding='utf-8') as json_file:
        json.dump(data, json_file, indent=4)  # Pretty print the JSON output

    print(f'Successfully converted {csv_file_path} to {json_file_path}')



# Every run of a model version writes into its own directory, and start.sh passes
# "<run dir>=<namespace>/<name>@<commit>" for each of them. A result is attributed by
# the directory its file sits under, so two repositories that share a basename and a
# commit stay distinct and nothing depends on how the framework named anything.
def parse_run_map(raw):
    runs = []
    for entry in (raw or "").split(","):
        run_dir, sep, target = entry.strip().partition("=")
        if not sep or not run_dir or not target:
            continue
        repo_id, _, revision = target.partition("@")
        if repo_id:
            runs.append((run_dir.rstrip("/") + "/", repo_id, revision))
    # longest first so /run1 never matches a path under /run10
    runs.sort(key=lambda r: len(r[0]), reverse=True)
    return runs


def run_of(runs, report_path):
    path = str(report_path)
    for prefix, repo_id, revision in runs:
        if path.startswith(prefix):
            return repo_id, revision
    return "", ""


column = [
    {
        "title": {
            "zh-CN": "数据集",
            "en-US": "Dataset"
        },
        "width": 220,
        "key": "dataset",
        "fixed": "left"
    },
    {
        "title": {
            "zh-CN": "指标",
            "en-US": "Metric"
        },
        "width": 130,
        "key": "metric",
        "fixed": "left"
    },
    {
        "title": {
            "zh-CN": "模式",
            "en-US": "Mode"
        },
        "width": 100,
        "key": "mode",
        "fixed": "left"
    },
    {
        "title": {
            "zh-CN": "模型",
            "en-US": "Model"
        },
        "width": 220,
        "key": "model",
        "fixed": "left"
    },
    {
        "title": {
            "zh-CN": "版本",
            "en-US": "Revision"
        },
        "width": 120,
        "key": "revision",
        "fixed": "left"
    },
    {
        "title": {
            "zh-CN": "评分",
            "en-US": "Score"
        },
        "width": 100,
        "key": "score",
        "fixed": "left"
    }
]


def json_to_summary(jsonPath, tasks, run_map_raw=''):
    runs = parse_run_map(run_map_raw)
    summary_data = []
    xlsx_json = {}
    final_json={}
    for index, jsonPath in enumerate(jsonPath):
        with open(jsonPath, 'r', encoding='utf-8') as f:
            jsonObj = json.load(f)
        repo_id, revision = run_of(runs, jsonPath)
        keywords = tasks
        # generate summary data
        for item in jsonObj:
            item_new = item.copy()
            if item_new["dataset"] in keywords:
                keys = list(item_new.keys())
                model_name = keys[-1]
                item_new['id'] = len(summary_data) + 1
                item_new['score']= item_new[model_name]
                item_new['repo_id']=repo_id
                item_new['model']=repo_id.rsplit('/', 1)[-1] if repo_id else model_name
                item_new['revision']=revision
                summary_data.append(item_new)
        summary = {
            "column": column,
            "data": summary_data
        }
        final_json['summary'] = summary
        xlsx_json['summary'] = summary_data
        # generate detail data
        for item in keywords:
            sub_data = []
            for sub_item in jsonObj:
                item_new = sub_item.copy()
                if item in item_new["dataset"]:
                    keys = list(item_new.keys())
                    model_name = keys[-1]
                    item_new['id'] = len(sub_data) + 1
                    item_new['score']= item_new[model_name]
                    item_new['repo_id']=repo_id
                    item_new['model']=repo_id.rsplit('/', 1)[-1] if repo_id else model_name
                    item_new['revision']=revision
                    sub_data.append(item_new)
            if item in xlsx_json:
                xlsx_json[item].extend(sub_data)
                final_json[item] = {"column": column, "data": xlsx_json[item]}
            else:
                xlsx_json[item] = sub_data
                final_json[item] = {"column": column, "data": sub_data}

    final_path="/workspace/output/final/"
    json_file_path = final_path + 'upload.json'
    with open(json_file_path, 'w', encoding='utf-8') as f:
        json.dump(final_json, f, ensure_ascii=False, indent=4)

    xlsx_file = final_path + 'upload.xlsx'
    with pd.ExcelWriter(xlsx_file) as writer:
        for sheet_name, records in xlsx_json.items():
            df = pd.DataFrame(records)
            df.to_excel(writer, sheet_name=sheet_name, index=False)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Get upload files.')
    subparsers = parser.add_subparsers(dest='command', required=True)

    parser_a = subparsers.add_parser('upload', help='upload files')
    parser_a.add_argument('files', type=str, help='Name to greet')

    parser_b = subparsers.add_parser('convert', help='Convert csv to json')
    parser_b.add_argument('file', type=str, help='Convert csv to json')

    parser_c = subparsers.add_parser('summary', help='Convert json to json summary')
    parser_c.add_argument('--file',nargs='+', type=str, help='Convert json to json summary')
    parser_c.add_argument('--tasks', nargs='+', type=str, help='task list')
    parser_c.add_argument('--run-map', dest='run_map', default='', type=str,
                          help='comma separated <run dir>=<namespace>/<name>@<commit> entries')

    args = parser.parse_args()

    if args.command == 'upload':
        upload(args.files)
    elif args.command == 'convert':
        csv_to_json(args.file)
    elif args.command == 'summary':
        json_to_summary(args.file, args.tasks, args.run_map)
