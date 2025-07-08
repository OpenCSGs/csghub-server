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
            "zh-CN": "评分",
            "en-US": "Score"
        },
        "width": 100,
        "key": "score",
        "fixed": "left"
    }
]


def get_metric_value(resultObj, key, metric):
    for item, value in resultObj[key].items():
        if metric in item:
            return round(value*100, 2)


def get_metric_items(jsonObj, item, model_name):
    metric_items = []
    metrics = jsonObj['higher_is_better'][item].keys()
    for metric in metrics:
        value = get_metric_value(jsonObj['results'], item, metric)
        item_new = {
            "dataset": item,
            "version": jsonObj['versions'][item],
            "metric": metric,
            "model":model_name,
            "score": value,
        }
        metric_items.append(item_new)
    return metric_items


def is_a_sub_group(jsonObj, key):
    for k, v in jsonObj["group_subtasks"].items():
        if key in v:
            return True
    return False


def json_to_summary(jsonPaths, model_names):
    
    summary_data = []
    xlsx_json = {}
    final_json={}
    json_paths = jsonPaths.split(',')
    names = model_names.split(',')
    for index, jsonPath in enumerate(json_paths):
        model_name=names[index]
        with open(jsonPath, 'r', encoding='utf-8') as f:
            jsonObj = json.load(f)
        # generate summary data
        for k in jsonObj["group_subtasks"].keys():
            m_items = get_metric_items(jsonObj, k, model_name)
            summary_data.extend(m_items)
        summary = {
            "column": column,
            "data": summary_data
        }
        final_json['summary'] = summary
        xlsx_json['summary'] = summary_data
        # generate detail data
        for key, value in jsonObj["group_subtasks"].items():
            sub_data = []
            if len(value) == 0:
                m_items = get_metric_items(jsonObj, key, model_name)
                sub_data.extend(m_items)
                if key in xlsx_json:
                    xlsx_json[key].extend(sub_data)
                    final_json[key] = {"column": column, "data": xlsx_json[key]}
                else:
                    xlsx_json[key] = sub_data
                    final_json[key] = {"column": column, "data": sub_data}
            else:
                sub_g = is_a_sub_group(jsonObj, key)
                if not sub_g:
                    # loop root group
                    for sub_key in value:
                        m_items = get_metric_items(jsonObj, sub_key, model_name)
                        sub_data.extend(m_items)
                        if sub_key not in jsonObj["group_subtasks"]:
                            continue
                        # level 2 group
                        value2 = jsonObj["group_subtasks"][sub_key]
                        if len(value2) == 0:
                            m_items = get_metric_items(jsonObj, key, model_name)
                            sub_data.extend(m_items)
                        else:
                            for sub_sub_key in value2:
                                m_items = get_metric_items(jsonObj, sub_sub_key, model_name)
                                sub_data.extend(m_items)
                    # check xlsx_json[key] exists
                    if key in xlsx_json:
                        xlsx_json[key].extend(sub_data)
                        final_json[key] = {"column": column, "data": xlsx_json[key]}
                    else:
                        xlsx_json[key] = sub_data
                        final_json[key] = {"column": column, "data": sub_data}
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

    parser_c = subparsers.add_parser('summary', help='Convert json to json summary')
    parser_c.add_argument('--file', type=str, help='Convert json to json summary')
    parser_c.add_argument('--model', type=str, help='model name')

    args = parser.parse_args()

    if args.command == 'upload':
        upload(args.files)
    elif args.command == 'summary':
        json_to_summary(args.file.strip(','), args.model.strip(','))
