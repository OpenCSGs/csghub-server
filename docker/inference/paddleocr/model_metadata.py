import argparse

import yaml


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("metadata_path")
    args = parser.parse_args()

    with open(args.metadata_path) as metadata_file:
        metadata = yaml.safe_load(metadata_file) or {}

    model_name = metadata.get("Global", {}).get("model_name")
    if not isinstance(model_name, str) or not model_name.strip():
        raise SystemExit(f"ERROR: {args.metadata_path} has no Global.model_name")

    print(model_name.strip())


if __name__ == "__main__":
    main()
