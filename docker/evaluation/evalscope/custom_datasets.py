"""
Custom dataset registration for evalscope v1.1.1+
This module registers custom datasets that are not included in evalscope by default.
"""

try:
    # Evalscope v1.1.1+ API
    from evalscope.api.benchmark import DefaultDataAdapter
    from evalscope.api.benchmark.meta import BenchmarkMeta
    from evalscope.api.registry import BENCHMARK_REGISTRY, register_benchmark
    from evalscope.api.dataset import Sample  # Correct import path
    EVALSCOPE_AVAILABLE = True
    print("[DEBUG] Successfully imported evalscope v1.1.1+ API")
except ImportError as e:
    print(f"[ERROR] Failed to import evalscope: {e}")
    print("[ERROR] Make sure evalscope v1.1.1+ is installed in the container")
    print("[ERROR] Custom datasets will not be registered")
    EVALSCOPE_AVAILABLE = False
    import traceback
    traceback.print_exc()


class CivilCommentsDataAdapter(DefaultDataAdapter):
    """
    Custom data adapter for civil_comments datasets

    The civil_comments dataset contains text samples with toxicity labels.
    Expected fields in the dataset:
    - text: The comment text to classify
    - toxicity: The toxicity label (binary: 0 or 1, or continuous: 0.0 to 1.0)
    - Other optional fields: severe_toxicity, obscene, threat, insult, identity_attack, sexual_explicit
    """

    def format_prompt_template(self, sample: Sample) -> str:
        """
        Override to skip additional prompt formatting since we already format in record_to_sample

        Args:
            sample: The Sample object with pre-formatted input

        Returns:
            The sample.input as-is without additional formatting
        """
        return sample.input

    def record_to_sample(self, record: dict) -> Sample:
        """
        Convert a dataset record to evalscope Sample format

        Args:
            record: A dictionary containing the dataset record with 'text' and 'toxicity' fields

        Returns:
            Sample object with the converted data
        """
        # Extract text and toxicity label
        text = record.get('text', '')
        toxicity = record.get('toxicity', 0)

        # Convert toxicity to binary label if it's a float (threshold at 0.5)
        if isinstance(toxicity, float):
            label = 1 if toxicity >= 0.5 else 0
        else:
            label = int(toxicity)

        # Create a simple classification prompt
        # The model should classify whether the comment is toxic or not
        prompt = f"Classify the following comment as toxic (1) or not toxic (0):\n\nComment: {text}\n\nClassification:"

        # Return Sample with the correct field structure
        return Sample(
            input=prompt,  # Required: the input prompt for the model
            target=str(label),  # Optional: the reference/ground truth answer
            metadata={  # Optional: additional metadata
                'text': text,
                'toxicity': toxicity,
            }
        )


def register_custom_datasets():
    """
    Register all custom datasets with evalscope

    Reads DATASET_IDS from environment variable and registers matching custom datasets.
    For example:
    - DATASET_IDS="google/civil_comments" -> registers google/civil_comments
    - DATASET_IDS="abc/civil_comments" -> registers abc/civil_comments
    - DATASET_IDS="abc/civil_comments,test/hellaswag" -> registers abc/civil_comments
    """

    print("[DEBUG] register_custom_datasets() called")

    if not EVALSCOPE_AVAILABLE:
        print("[WARNING] Evalscope not available, skipping custom dataset registration")
        return

    import os

    print("[DEBUG] EVALSCOPE_AVAILABLE = True")

    # Read DATASET_IDS from environment variable
    dataset_ids = os.environ.get('DATASET_IDS', '')
    print(f"[DEBUG] Reading DATASET_IDS from environment: '{dataset_ids}'")

    if not dataset_ids:
        print("[WARNING] No DATASET_IDS found in environment, skipping custom dataset registration")
        return

    # Parse dataset IDs (comma-separated)
    dataset_id_list = [ds.strip() for ds in dataset_ids.split(',')]
    print(f"[DEBUG] Parsed dataset_id_list: {dataset_id_list}")

    # Register civil_comments datasets
    registered_count = 0
    for dataset_id in dataset_id_list:
        print(f"[DEBUG] Processing dataset_id: '{dataset_id}'")
        if 'civil_comments' in dataset_id.lower():
            print(f"[DEBUG] Found civil_comments dataset: '{dataset_id}'")
            try:
                # Use the full dataset_id as the benchmark name to avoid conflicts
                # e.g., "google_civil_comments" or "James_civil_comments"
                benchmark_name = dataset_id.replace('/', '_').replace('-', '_')

                print(f"[DEBUG] Benchmark name: '{benchmark_name}'")

                # Create BenchmarkMeta with CivilCommentsDataAdapter
                civil_comments_meta = BenchmarkMeta(
                    name=benchmark_name,
                    dataset_id=dataset_id,
                    data_adapter=CivilCommentsDataAdapter,
                    subset_list=['default'],
                    metric_list=['acc'],  # Use 'acc' instead of 'accuracy' - it's registered in evalscope
                    few_shot_num=0,
                    train_split=None,
                    eval_split='test',
                )

                print(f"[DEBUG] Created BenchmarkMeta: name='{benchmark_name}', dataset_id='{dataset_id}'")

                # Register benchmark using BENCHMARK_REGISTRY
                BENCHMARK_REGISTRY[benchmark_name] = civil_comments_meta

                print(f"✓ Custom dataset '{dataset_id}' registered successfully as '{benchmark_name}'")
                registered_count += 1
            except Exception as e:
                print(f"[ERROR] Failed to register {dataset_id}: {e}")
                import traceback
                traceback.print_exc()
        else:
            print(f"[DEBUG] Skipping '{dataset_id}' (not a civil_comments dataset)")

    print(f"[INFO] Total civil_comments datasets registered: {registered_count}")


if __name__ == '__main__':
    register_custom_datasets()
