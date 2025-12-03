#!/usr/bin/env python3
"""
Script to register custom datasets with evalscope before evaluation
"""

import sys
import os

# Add custom datasets path to Python path
sys.path.insert(0, '/etc/csghub')

print(f"[DEBUG] register_custom.py starting...")
print(f"[DEBUG] DATASET_IDS environment variable: {os.environ.get('DATASET_IDS', 'NOT SET')}")

try:
    from custom_datasets import register_custom_datasets
    print(f"[DEBUG] Calling register_custom_datasets()...")
    register_custom_datasets()
    print("[SUCCESS] Custom datasets registered successfully")
except Exception as e:
    print(f"[ERROR] Failed to register custom datasets: {e}")
    import traceback
    traceback.print_exc()
    # Don't fail the entire evaluation if custom registration fails
    sys.exit(0)
