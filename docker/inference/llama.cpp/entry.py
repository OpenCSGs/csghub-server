from pycsghub.snapshot_download import snapshot_download
import os
import shlex


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


if __name__ == "__main__":
    model_file = get_argument_value(os.environ['ENGINE_ARGS'], '-m')
    if model_file is None:
        model_file = os.environ['GGUF_ENTRYPOINT']
    model_file = model_file.replace("00001-of", "*")
    DOWNLOAD_DIR = "/workspace"
    REPO_ID = os.environ['REPO_ID']
    REVISION = os.getenv('REVISION', 'main')
    TOKEN = os.environ['ACCESS_TOKEN']
    ENDPOINT = os.environ['HF_ENDPOINT']
    os.environ['CSGHUB_DOMAIN'] = ENDPOINT
    snapshot_download(REPO_ID, cache_dir=DOWNLOAD_DIR, endpoint=ENDPOINT, token=TOKEN, allow_patterns=model_file)
