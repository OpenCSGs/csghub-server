import argparse
from opencompass.utils.datasets_info import DATASETS_MAPPING


def get_dataset_key_by_ms_id(ms_id):
    for key, value in DATASETS_MAPPING.items():
        if value['ms_id'] == ms_id:
            return key
    return None


def main(ms_id):
    names = ms_id.split("/")
    # Extract the last part
    name = names[-1]
    # Generate the new string
    opencompass_id = f"opencompass/{name}"
    dataset_key = get_dataset_key_by_ms_id(opencompass_id)
    if dataset_key:
        print(f"{dataset_key}")
    else:
        return


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Get dataset key by ms_id.')
    parser.add_argument('ms_id', type=str, help='The ms_id to search for')
    args = parser.parse_args()

    main(args.ms_id)
