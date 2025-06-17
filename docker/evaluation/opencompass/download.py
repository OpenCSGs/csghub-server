import argparse
from pathlib import Path
from huggingface_hub import snapshot_download

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Download repo.')
    subparsers = parser.add_subparsers(dest='command', required=True)

    parser_a = subparsers.add_parser('datasets', help='download datasets')
    parser_a.add_argument('--dataset_ids', type=str, help='repo id')
    parser_a.add_argument('--endpoint', type=str, help='endpoint')
    parser_a.add_argument('--token', type=str, help='token')
    parser_a.add_argument('--revision', type=str, help='revision')
    parser_b = subparsers.add_parser('models', help='download model')
    parser_b.add_argument('--model_ids', type=str, help='repo id')
    parser_b.add_argument('--endpoint', type=str, help='endpoint')
    parser_b.add_argument('--token', type=str, help='token')
    parser_b.add_argument('--revision', type=str, help='revision')

    args = parser.parse_args()
    endpoint= args.endpoint
    token = args.token
    revision = args.revision
    # split repo ids
    if args.command == 'models':
        repo_ids = args.model_ids.split(',')
        for repo_id in repo_ids:
            snapshot_download(repo_id=repo_id, ignore_patterns=["*.txt", "*.bin"], endpoint=endpoint, token=token,local_dir="/workspace/"+repo_id, revision=revision)
    elif args.command == 'datasets':
        repo_ids = args.dataset_ids.split(',')
        for repo_id in repo_ids:
            snapshot_download(repo_id=repo_id, repo_type="dataset", endpoint=endpoint, token=token, local_dir="/workspace/data/"+repo_id, revision=revision,ignore_patterns=["dataset_infos.json"])
