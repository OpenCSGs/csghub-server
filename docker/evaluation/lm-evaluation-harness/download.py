import os
import argparse
import datasets
# Load model directly
from huggingface_hub import snapshot_download

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Download repo.')
    subparsers = parser.add_subparsers(dest='command', required=True)

    parser_a = subparsers.add_parser('datasets', help='download datasets')
    parser_a.add_argument('--dataset_ids', type=str, help='repo id')
    parser_b = subparsers.add_parser('models', help='download model')
    parser_b.add_argument('--model_ids', type=str, help='repo id')

    args = parser.parse_args()

    # split repo ids

    if args.command == 'models':
        repo_ids = args.model_ids.split(',')
        for repo_id in repo_ids:
            snapshot_download(repo_id=repo_id)
    elif args.command == 'datasets':
        repo_ids = args.dataset_ids.split(',')
        for repo_id in repo_ids:
            snapshot_download(repo_id=repo_id, repo_type="dataset")
