import argparse
from evalscope.benchmarks.benchmark import BENCHMARK_MAPPINGS

def find_name_by_dataset_id(target_dataset_id):
    for name, meta in BENCHMARK_MAPPINGS.items():
        if meta.dataset_id == target_dataset_id:
            return name
    return ""

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Get dataset key by ms_id.')
    parser.add_argument('ms_id', type=str, help='The ms_id to search for')
    args = parser.parse_args()
    task = find_name_by_dataset_id(args.ms_id)
    with open("/tmp/task.txt", "w", encoding="utf-8") as f:
        f.write(task)