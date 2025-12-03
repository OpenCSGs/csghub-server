#!/usr/bin/env python3
"""
Wrapper script for evalscope that registers custom datasets before running.
"""
from evalscope.cli.cli import run_cmd
import sys
import os

# Add custom datasets path to Python path
sys.path.insert(0, '/etc/csghub')

# Register custom datasets BEFORE importing evalscope
try:
    from custom_datasets import register_custom_datasets
    register_custom_datasets()
    print("[INFO] Custom datasets registered successfully")
except Exception as e:
    print(f"[WARNING] Failed to register custom datasets: {e}")
    import traceback
    traceback.print_exc()

# Now run evalscope CLI

if __name__ == '__main__':
    sys.exit(run_cmd())
