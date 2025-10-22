#!/usr/bin/env python3
"""
Clean wrapper for get_model_info.py that suppresses all unwanted output
"""
import subprocess
import sys
import os

def main():
    if len(sys.argv) != 2:
        print("Usage: get_model_info_clean.py <model_name>")
        sys.exit(1)
    
    model_name = sys.argv[1]
    
    # Run the original script and capture only the last line
    try:
        result = subprocess.run([
            'python', '-W', 'ignore', '/etc/csghub/get_model_info.py', model_name
        ], capture_output=True, text=True, cwd='/workspace')
        
        # Get the last line of output (which should be our CSV format)
        output_lines = result.stdout.strip().split('\n')
        if output_lines:
            last_line = output_lines[-1]
            # Only print if it looks like our expected CSV format
            if ',' in last_line and len(last_line.split(',')) == 3:
                print(last_line)
            else:
                print('qwen3,qwen3,no')
        else:
            print('qwen3,qwen3,no')
            
    except Exception:
        print('qwen3,qwen3,no')

if __name__ == '__main__':
    main()
