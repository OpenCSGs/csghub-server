from typing import Any, List
import argparse
from swift.llm import MODEL_MAPPING, TEMPLATE_MAPPING, ModelType, TemplateType


def get_url_suffix(model_id):
    if ':' in model_id:
        return model_id.split(':')[0]
    return model_id


def generate_model_sql():
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
                # check namespace_and_name contains ':'
                if ":" in namespace_and_name[1]:
                    continue
                # generate sql and save to file
                sql = f"\"{namespace_and_name[1]}\","
                with open("resource_model.txt", 'a') as file:
                    file.write(sql + '\n')


if __name__ == '__main__':
    generate_model_sql()
