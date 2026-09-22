import os
import argparse
import re
from datetime import datetime
from minio import Minio
from minio.error import S3Error
from pathlib import Path
import pandas as pd
import json
import oss2

access_key_id = os.environ['S3_ACCESS_ID']
access_key_secret = os.environ['S3_ACCESS_SECRET']
bucket_name = os.environ['S3_BUCKET']
endpoint = os.environ['S3_ENDPOINT']
s3_ssl_enabled = json.loads(os.environ['S3_SSL_ENABLED'])
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


def upload(files):
    output = []
    # get schema based on s3_ssl_enabled
    schema = "https" if s3_ssl_enabled else "http"
    fileName = generate_file_name("result")
    for file in files.split(','):
        suffix = Path(file).suffix
        object_name = f"evaluation/{fileName}{suffix}"
        # check if the endpoint is aliyun oss
        if endpoint.find("aliyuncs.com") != -1:
            upload_to_ali(object_name, file)
            file_url = f"https://{bucket_name}.{endpoint}/{object_name}"
        else:
            upload_to_minio(object_name, file)
            file_url = f"{schema}://{endpoint}/{bucket_name}/{object_name}"
        output.append(file_url)
    try:
        with open('/tmp/output.txt', 'w') as file:
            file.write(",".join(output))
            print("Output written to /tmp/output.txt")
        print(f'Successfully uploaded to {file_url}')
    except Exception as e:
        print(f"Error writing to file: {e}")



# Every run of a model version writes into its own directory, and start.sh passes
# "<run dir>=<namespace>/<name>@<commit>" for each of them. A report is attributed by
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


# evalscope 1.11 reshaped its report: a metric no longer carries a plain "name"
# but an identity plus a legacy name, and the report itself no longer has a
# top-level score. Read both shapes so one script works across image versions.
def metric_name(metric):
    identity = metric.get("identity") or {}
    return metric.get("legacy_name") or identity.get("name") or metric.get("name") or ""


def report_score(report):
    if "score" in report:
        return report["score"]
    metrics = report.get("metrics") or []
    return metrics[0].get("score") if metrics else None

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
            "zh-CN": "模型仓库",
            "en-US": "Repository"
        },
        "width": 260,
        "key": "repo_id",
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
        # generate summary data
        item_new={}
        repo_id, revision = run_of(runs, jsonPath)
        # the repository is the source of truth for the display name; fall back to
        # what the framework reported only when a run cannot be attributed
        model_display = repo_id.rsplit('/', 1)[-1] if repo_id else jsonObj.get('model_name', '')
        task = jsonObj['dataset_name']
        item_new['repo_id']=repo_id
        item_new['model']=model_display
        item_new['revision']=revision
        item_new['dataset']=task
        metrics = jsonObj.get('metrics') or []
        item_new['metric']=metric_name(metrics[0]) if metrics else ''
        item_new['score']=report_score(jsonObj)
        summary_data.append(item_new)
        summary = {
            "column": column,
            "data": summary_data
        }
        final_json['summary'] = summary
        xlsx_json['summary'] = summary_data
        # generate detail data
        sub_data = []
        for metric in (jsonObj.get('metrics') or []):
            for category in (metric.get('categories') or []):
                for subset in (category.get('subsets') or []):
                    subset_data={}
                    subset_data['repo_id']=repo_id
                    subset_data['model']=model_display
                    subset_data['revision']=revision
                    subset_data['dataset']=subset['name']
                    subset_data['metric']=metric_name(metric)
                    subset_data['score']=subset['score']
                    sub_data.append(subset_data)
        if task in xlsx_json:
            xlsx_json[task].extend(sub_data)
            final_json[task] = {"column": column, "data": xlsx_json[task]}
        else:
            xlsx_json[task] = sub_data
            final_json[task] = {"column": column, "data": sub_data}

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
    parser_c.add_argument('--run-map', dest='run_map', type=str, default='',
                          help='comma separated <run dir>=<namespace>/<name>@<commit> entries')

    args = parser.parse_args()

    if args.command == 'upload':
        upload(args.files)
    elif args.command == 'summary':
        json_to_summary(args.file, args.tasks, args.run_map)
