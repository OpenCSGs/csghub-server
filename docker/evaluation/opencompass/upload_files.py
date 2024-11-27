import os
import argparse
from datetime import datetime
from minio import Minio
from minio.error import S3Error
from pathlib import Path
import pandas as pd
import json
import oss2
import csv

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
    }
]


def json_to_summary(jsonPath, tasks):
    with open(jsonPath, 'r', encoding='utf-8') as f:
        jsonObj = json.load(f)
    keywords = tasks
    summary_data = []
    model_name = ""
    xlsx_json = {}
    # generate summary data
    for item in jsonObj:
        item_new = item.copy()
        if item_new["dataset"] in keywords:
            keys = list(item_new.keys())
            model_name = item_new[keys[-1]]
            item_new['id'] = len(summary_data) + 1
            summary_data.append(item_new)
    column.append(
        {
            "title": {
                "zh-CN": model_name,
                "en-US": model_name
            },
            "width": 200,
            "key": model_name,
            "customizeRender": {
                "sorter": "descend"
            }
        }
    )
    summary = {
        "column": column,
        "data": summary_data
    }
    final_json = {"summary": summary}
    xlsx_json['summary'] = summary_data
    # generate detail data
    for item in keywords:
        sub_data = []
        for sub_item in jsonObj:
            item_new = sub_item.copy()
            if item in item_new["dataset"]:
                item_new['id'] = len(sub_data) + 1
                sub_data.append(item_new)
        xlsx_json[item] = sub_data
        final_json[item] = {"column": column, "data": sub_data}

    json_file_path = os.path.splitext(jsonPath)[0] + '_upload.json'
    with open(json_file_path, 'w', encoding='utf-8') as f:
        json.dump(final_json, f, ensure_ascii=False, indent=4)

    xlsx_file = os.path.splitext(jsonPath)[0] + '_upload.xlsx'
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
    parser_c.add_argument('--file', type=str, help='Convert json to json summary')
    parser_c.add_argument('--tasks', nargs='+', type=str, help='task list')

    args = parser.parse_args()

    if args.command == 'upload':
        upload(args.files)
    elif args.command == 'convert':
        csv_to_json(args.file)
    elif args.command == 'summary':
        json_to_summary(args.file, args.tasks)
