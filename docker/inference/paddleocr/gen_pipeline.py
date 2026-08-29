"""Patch a PaddleX OCR pipeline config to use the downloaded repo's weights.

The CSGHub repo is a single recognition model (e.g. PP-OCRv6_small_rec):
TextRecognition is pointed at the local weights and TextDetection is switched
to the matching-tier det model name (still resolved from the model sources).
Language-prefixed rec repos (latin_/korean_/eslav_) map to the unprefixed det.
"""

import argparse

LANG_PREFIXES = ("latin_", "korean_", "eslav_")


def patch_classic_ocr(cfg, model_name, model_dir):
    sub_modules = cfg.get("SubModules", {})

    if "TextRecognition" in sub_modules:
        sub_modules["TextRecognition"]["model_name"] = model_name
        sub_modules["TextRecognition"]["model_dir"] = model_dir

    det_name = model_name
    for prefix in LANG_PREFIXES:
        if det_name.startswith(prefix):
            det_name = det_name[len(prefix):]
            break
    det_name = det_name.replace("_rec", "_det")
    if "TextDetection" in sub_modules and det_name != model_name:
        sub_modules["TextDetection"]["model_name"] = det_name
        sub_modules["TextDetection"]["model_dir"] = None


def patch_paddleocr_vl(cfg, model_name, model_dir):
    sub_modules = cfg.get("SubModules", {})
    vl_recognition = sub_modules.get("VLRecognition")
    if vl_recognition is None:
        raise ValueError("PaddleOCR-VL pipeline config has no SubModules.VLRecognition")

    vl_recognition["model_name"] = model_name
    vl_recognition["model_dir"] = model_dir


def main():
    import yaml

    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--pipeline", choices=("ocr", "paddleocr-vl"), default="ocr")
    parser.add_argument("--rec-name", required=True)
    parser.add_argument("--rec-dir", required=True)
    args = parser.parse_args()

    with open(args.config) as f:
        cfg = yaml.safe_load(f)

    if args.pipeline == "paddleocr-vl":
        patch_paddleocr_vl(cfg, args.rec_name, args.rec_dir)
    else:
        patch_classic_ocr(cfg, args.rec_name, args.rec_dir)

    with open(args.config, "w") as f:
        yaml.safe_dump(cfg, f, sort_keys=False)


if __name__ == "__main__":
    main()
