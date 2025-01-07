from typing import Any, List
import argparse
from swift.llm import MODEL_MAPPING, TEMPLATE_MAPPING, ModelType, TemplateType


def get_url_suffix(model_id):
    if ':' in model_id:
        return model_id.split(':')[0]
    return model_id


def get_model_info(model_name: str):
    for template in TemplateType.get_template_name_list():
        assert template in TEMPLATE_MAPPING

    for model_type in ModelType.get_model_name_list():
        model_meta = MODEL_MAPPING[model_type]
        template = model_meta.template
        for group in model_meta.model_groups:
            for model in group.models:
                hf_model_id = model.hf_model_id
                if hf_model_id is None:
                    continue
                namespace_and_name = hf_model_id.split('/')
                if namespace_and_name[1] == model_name:
                    requires = ', '.join(group.requires or model_meta.requires) or '-'
                    lower_transformers = "yes" if '<' in requires else "no"
                    print(f'{model_type},{template},{lower_transformers}')
                    break


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Get dataset key by model_name.')
    parser.add_argument('model_name', type=str, help='The model_name to search for')
    args = parser.parse_args()
    get_model_info(args.model_name)
