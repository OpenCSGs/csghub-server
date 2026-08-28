from typing import Any, List
import argparse
import os
import logging
import sys
import contextlib

# Suppress all logging before importing swift modules
os.environ['TRANSFORMERS_VERBOSITY'] = 'error'
os.environ['TOKENIZERS_PARALLELISM'] = 'false'
os.environ['ACCELERATE_LOG_LEVEL'] = 'error'
os.environ['CUDA_VISIBLE_DEVICES'] = ''  # Disable CUDA to prevent accelerator detection

# Set up logging suppression
logging.getLogger().setLevel(logging.CRITICAL)

# Suppress specific loggers that might output unwanted information
logging.getLogger('transformers').setLevel(logging.CRITICAL)
logging.getLogger('torch').setLevel(logging.CRITICAL)
logging.getLogger('deepspeed').setLevel(logging.CRITICAL)
logging.getLogger('accelerate').setLevel(logging.CRITICAL)

# Suppress warnings
import warnings
warnings.filterwarnings('ignore')

# Context manager to suppress both stdout and stderr during imports
@contextlib.contextmanager
def suppress_output():
    with open(os.devnull, "w") as devnull:
        old_stdout = sys.stdout
        old_stderr = sys.stderr
        sys.stdout = devnull
        sys.stderr = devnull
        try:
            yield
        finally:
            sys.stdout = old_stdout
            sys.stderr = old_stderr

# Import swift modules with all output suppressed
with suppress_output():
    from swift.model import MODEL_MAPPING, ModelType
    from swift.template import TEMPLATE_MAPPING, TemplateType


def get_url_suffix(model_id):
    if ':' in model_id:
        return model_id.split(':')[0]
    return model_id


def get_model_info(model_name: str):
    try:
        for template in TemplateType.get_template_name_list():
            assert template in TEMPLATE_MAPPING

        for model_type in ModelType.get_model_name_list():
            model_meta = MODEL_MAPPING[model_type]
            for group in model_meta.model_groups:
                template = group.template or model_meta.template
                for model in group.models:
                    hf_model_id = model.hf_model_id
                    if hf_model_id is None:
                        continue
                    namespace_and_name = hf_model_id.split('/')
                    if len(namespace_and_name) >= 2 and namespace_and_name[1] == model_name:
                        requires = ', '.join(group.requires or model_meta.requires) or '-'
                        lower_transformers = "yes" if '<' in requires else "no"
                        print(f'{model_type},{template},{lower_transformers}')
                        return
        # Let ms-swift auto-detect unknown models instead of applying an
        # incorrect Qwen model type and template.
        print(',,no')
    except Exception:
        print(',,no')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Get dataset key by model_name.')
    parser.add_argument('model_name', type=str, help='The model_name to search for')
    args = parser.parse_args()
    get_model_info(args.model_name)
