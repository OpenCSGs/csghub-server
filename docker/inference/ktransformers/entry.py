from pycsghub.snapshot_download import snapshot_download
import os
import shlex
import argparse


def get_argument_value(arg_string: str, target_arg: str) -> str:
    """
    Parse command line style arguments and retrieve the value of the specified target argument.

    Args:
        arg_string (str): The argument string to parse.
        target_arg (str): The target argument to find the value for.

    Returns:
        str: The value associated with the target argument or None if not found.
    """
    # Split the argument string into components
    args = shlex.split(arg_string)

    # Initialize variable for the value
    target_value = None

    # Iterate through arguments to find the target
    for i in range(len(args)):
        if args[i] == target_arg and i + 1 < len(args):
            target_value = args[i + 1]  # Get the next argument
            break

    return target_value


def download_model(repo_id, allow_patterns):
    # download gguf file
    DOWNLOAD_DIR = "/workspace"
    TOKEN = os.environ['ACCESS_TOKEN']
    REVISION = os.getenv('REVISION', 'main')
    ENDPOINT = os.environ['HF_ENDPOINT']
    snapshot_download(repo_id, revision=REVISION, cache_dir=DOWNLOAD_DIR,
                      endpoint=ENDPOINT, token=TOKEN, allow_patterns=allow_patterns)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='download repo')
    parser.add_argument('--repo_id', type=str, help='The repo_id')
    parser.add_argument('--model_format', type=str, help='The model_format')
    args = parser.parse_args()
    if args.model_format == 'gguf':
        engine_args = os.environ.get('ENGINE_ARGS')
        model_file = None
        if engine_args is not None:
            model_file = get_argument_value(os.environ['ENGINE_ARGS'], '-m')
        if model_file is None:
            model_file = os.environ['GGUF_ENTRY_POINT']
        model_file = model_file.replace("00001-of", "*")
        allow_patterns = [
            model_file,
            "README.md"
        ]
        download_model(args.repo_id, allow_patterns)
    else:
        download_model(args.repo_id, "*.json")
