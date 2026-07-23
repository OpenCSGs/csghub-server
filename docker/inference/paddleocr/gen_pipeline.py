"""Patch a PaddleX OCR pipeline config to use the downloaded repo's weights.

The CSGHub repo is a single recognition model (e.g. PP-OCRv6_small_rec):
TextRecognition is pointed at the local weights and TextDetection is switched
to the matching-tier det model name (still resolved from the model sources).
Language-prefixed rec repos (latin_/korean_/eslav_) map to the unprefixed det.
"""

import argparse

import yaml

LANG_PREFIXES = ("latin_", "korean_", "eslav_")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--rec-name", required=True)
    parser.add_argument("--rec-dir", required=True)
    args = parser.parse_args()

    with open(args.config) as f:
        cfg = yaml.safe_load(f)

    sub_modules = cfg.get("SubModules", {})

    if "TextRecognition" in sub_modules:
        sub_modules["TextRecognition"]["model_name"] = args.rec_name
        sub_modules["TextRecognition"]["model_dir"] = args.rec_dir

    det_name = args.rec_name
    for prefix in LANG_PREFIXES:
        if det_name.startswith(prefix):
            det_name = det_name[len(prefix):]
            break
    det_name = det_name.replace("_rec", "_det")
    if "TextDetection" in sub_modules and det_name != args.rec_name:
        sub_modules["TextDetection"]["model_name"] = det_name
        sub_modules["TextDetection"]["model_dir"] = None

    with open(args.config, "w") as f:
        yaml.safe_dump(cfg, f, sort_keys=False)


if __name__ == "__main__":
    main()
