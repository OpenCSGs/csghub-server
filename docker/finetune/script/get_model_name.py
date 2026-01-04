#!/usr/bin/env python3
"""
Parse constants.py to find model_name by REPO_ID (DownloadSource.DEFAULT path).
Usage: python get_model_name.py <REPO_ID>
Example: python get_model_name.py "Qwen/Qwen3-0.6B"
Output: Qwen3-0.6B-Thinking
"""

import re
import sys

def find_model_name_by_repo_id(constants_file, repo_id):
    """
    Find model_name from constants.py based on REPO_ID (DownloadSource.DEFAULT path).
    
    The format in constants.py is:
    register_model_group(
        models={
            "ModelName": {
                DownloadSource.DEFAULT: "repo/path",
            },
        },
        template="...",
    )
    
    Args:
        constants_file: Path to constants.py file
        repo_id: The REPO_ID to search for (e.g., "Qwen/Qwen3-0.6B")
    
    Returns:
        The model_name if found, otherwise None
    """
    try:
        with open(constants_file, 'r', encoding='utf-8') as f:
            lines = f.readlines()
    except FileNotFoundError:
        return None
    
    # Search for the repo_id in DownloadSource.DEFAULT lines
    # Then look backwards to find the model name
    repo_pattern = rf'DownloadSource\.DEFAULT:\s*"{re.escape(repo_id)}"'
    
    for i, line in enumerate(lines):
        if re.search(repo_pattern, line):
            # Found the line with our repo_id, now look backwards for the model name
            # The model name should be on a line like: "ModelName": {
            for j in range(i, max(-1, i - 50), -1):  # Look back up to 50 lines
                model_match = re.search(r'"([^"]+)":\s*\{', lines[j])
                if model_match:
                    model_name = model_match.group(1)
                    # Verify this model dict contains our repo_id
                    # Get the content from this model line to the line with repo_id
                    model_dict_content = ''.join(lines[j:i+1])
                    if re.search(repo_pattern, model_dict_content):
                        return model_name
    
    return None

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python get_model_name.py <REPO_ID>", file=sys.stderr)
        sys.exit(1)
    
    repo_id = sys.argv[1]
    constants_file = "/app/src/llamafactory/extras/constants.py"
    
    model_name = find_model_name_by_repo_id(constants_file, repo_id)
    
    if model_name:
        print(model_name)
        sys.exit(0)
    else:
        # Fallback to extracting from REPO_ID if not found in constants.py
        # This maintains backward compatibility
        parts = repo_id.split('/')
        if len(parts) >= 2:
            print(parts[1])
        else:
            print(repo_id)
        sys.exit(0)

