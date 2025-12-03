import argparse
import sys
import os

# Register custom datasets first
sys.path.insert(0, '/etc/csghub')
try:
    from custom_datasets import register_custom_datasets
    register_custom_datasets()
except Exception as e:
    print(f"Warning: Failed to register custom datasets: {e}")
    import traceback
    traceback.print_exc()

try:
    from evalscope.api.registry import BENCHMARK_REGISTRY
    BENCHMARK_MAPPINGS = BENCHMARK_REGISTRY
    print("[DEBUG] Using BENCHMARK_REGISTRY from evalscope.api.registry")
except ImportError as e:
    print(f"[ERROR] Failed to import benchmark registry: {e}")
    BENCHMARK_MAPPINGS = {}

def find_name_by_dataset_id(target_dataset_id):
    """
    Find benchmark name by dataset_id.
    Returns the benchmark name if found, empty string otherwise.
    """
    print(f"[DEBUG] Searching for dataset_id: {target_dataset_id}")
    print(f"[DEBUG] Available benchmarks in registry: {len(BENCHMARK_MAPPINGS)} total")
    
    # First, try exact match
    for name, meta in BENCHMARK_MAPPINGS.items():
        if meta.dataset_id == target_dataset_id:
            print(f"[DEBUG] Found exact match: {name} -> {meta.dataset_id}")
            return name
    
    # If not found, print available civil_comments datasets for debugging
    print(f"[DEBUG] No exact match found for '{target_dataset_id}'")
    print(f"[DEBUG] Available civil_comments related benchmarks:")
    for name, meta in BENCHMARK_MAPPINGS.items():
        if 'civil' in name.lower() or 'civil' in str(meta.dataset_id).lower():
            print(f"[DEBUG]   - {name}: {meta.dataset_id}")
    
    return ""

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Get dataset key by ms_id.')
    parser.add_argument('ms_id', type=str, help='The ms_id to search for')
    args = parser.parse_args()
    
    print(f"[DEBUG] get_task.py called with ms_id: {args.ms_id}")
    print(f"[DEBUG] DATASET_IDS environment variable: {os.environ.get('DATASET_IDS', 'NOT SET')}")
    
    task = find_name_by_dataset_id(args.ms_id)
    
    if task:
        print(f"[SUCCESS] Found task name: {task}")
    else:
        print(f"[WARNING] No task found for dataset_id: {args.ms_id}")
    
    with open("/tmp/task.txt", "w", encoding="utf-8") as f:
        f.write(task)