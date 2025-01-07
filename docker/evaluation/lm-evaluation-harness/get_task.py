import collections
import argparse
import os

from lm_eval import utils


def _config_is_task(config) -> bool:
    if ("task" in config) and isinstance(config["task"], str):
        return True
    return False


def _config_is_group(config) -> bool:
    if ("task" in config) and isinstance(config["task"], list):
        return True
    return False


def _config_is_python_task(config) -> bool:
    if "class" in config:
        return True
    return False


def get_task_and_group(task_dir: str):
    def _populate_tags_and_groups(config, task, tasks_and_groups, print_info):
        # TODO: remove group in next release
        if "tag" in config:
            attr_list = config["tag"]
            if isinstance(attr_list, str):
                attr_list = [attr_list]

            for tag in attr_list:
                if tag not in tasks_and_groups:
                    tasks_and_groups[tag] = {
                        "type": "tag",
                        "task": [task],
                        "yaml_path": -1,
                    }
                elif tasks_and_groups[tag]["type"] != "tag":
                    break
                else:
                    tasks_and_groups[tag]["task"].append(task)

    # TODO: remove group in next release
    print_info = True
    ignore_dirs = [
        "__pycache__",
        ".ipynb_checkpoints",
    ]
    tasks_and_groups = collections.defaultdict()
    for root, dirs, file_list in os.walk(task_dir):
        dirs[:] = [d for d in dirs if d not in ignore_dirs]
        for f in file_list:
            if f.endswith(".yaml"):
                yaml_path = os.path.join(root, f)
                config = utils.load_yaml_config(yaml_path, mode="simple")
                if _config_is_python_task(config):
                    # This is a python class config
                    task = config["task"]
                    tasks_and_groups[task] = {
                        "type": "python_task",
                        "yaml_path": yaml_path,
                    }
                    _populate_tags_and_groups(
                        config, task, tasks_and_groups, print_info
                    )
                elif _config_is_group(config):
                    tasks_and_groups[config["group"]] = {
                        "type": "group",
                        "task": -1,
                        "yaml_path": yaml_path,
                    }

                elif _config_is_task(config):
                    # This is a task config
                    task = config["task"]
                    tasks_and_groups[task] = {
                        "type": "task",
                        "yaml_path": yaml_path,
                    }
                    _populate_tags_and_groups(
                        config, task, tasks_and_groups, print_info
                    )

    return tasks_and_groups


def get_related_task(task_dir, sub_task_yaml):
    tasks_and_groups = get_task_and_group(task_dir)
    # loop group
    all_groups = []
    all_tasks = []
    for task in tasks_and_groups:
        if tasks_and_groups[task]["type"] == "group":
            all_groups.append(task)
        if tasks_and_groups[task]["type"] == "task":
            if sub_task_yaml == tasks_and_groups[task]["yaml_path"]:
                return task
            all_tasks.append(task)
    if len(all_groups) > 0:
        sorted_tasks = sorted(all_groups, key=len)
        return sorted_tasks[0]
    else:
        return ", ".join(all_tasks)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Get dataset task by dir.')
    subparsers = parser.add_subparsers(dest='command', required=True)
    parser_c = subparsers.add_parser('task', help='Get task from path')
    parser_c.add_argument('--task_dir', type=str, help='Task dir')
    parser_c.add_argument('--sub_task_yaml', type=str, help='Sub task yaml')
    args = parser.parse_args()

    print(get_related_task(args.task_dir, args.sub_task_yaml))
