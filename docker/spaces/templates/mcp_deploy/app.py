import subprocess
from typing import Union, Any, Dict, List
from pathlib import Path
import os
import json
import tomli

def run_command(
    cmd: Union[str, List[str]],
) -> int:

    if isinstance(cmd, str):
        cmd = cmd.split()

    env = os.environ.copy()
    mirror_url = env.get("PYPI_INDEX_URL", None)
    if mirror_url:
        print(f"Using PyPI mirror URL: {mirror_url}")
        env.update({
            "PIP_INDEX_URL": mirror_url,
            "PIP_TRUSTED_HOST": mirror_url.split("//")[1].split("/")[0],
            "UV_INDEX_URL": mirror_url,
        })

    try:
        with subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            universal_newlines=True,
            bufsize=1,
            env=env,
        ) as process:
            
            for line in process.stdout:
                print(line, end='')
            
            return process.wait()
            
    except FileNotFoundError:
        print(f"Error: Command not found - {' '.join(cmd)}")
    except Exception as e:
        print(f"Execution failed: {str(e)}")
        return 1

def read_json_file(file_path: str) -> Dict[str, Any]:
    try:
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"JSON file not found: {file_path}")

        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
            
    except json.JSONDecodeError as e:
        raise json.JSONDecodeError(
            f"Invalid JSON format in file: {file_path}", 
            e.doc, 
            e.pos
        )

def get_mcp_cmds_from_json() -> Dict[str, Any]:
    config = {}
    try:
        config = read_json_file("mcp_space_conf.json")
        print("Successfully loaded JSON config:")
        print(json.dumps(config, indent=2, ensure_ascii=False))
    except Exception as e:
        print(f"Warn: {str(e)}")
    
    return config

def get_script_keys_from_toml(file_path: str) -> List[str]: 
    try:
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"TOML file not found: {file_path}")
            
        with open(path, "rb") as f:
            data = tomli.load(f)
            
        scripts = data.get("project", {}).get("scripts", {})
        return list(scripts.keys())
        
    except tomli.TOMLDecodeError as e:
        raise tomli.TOMLDecodeError(f"Invalid TOML format in file: {file_path}") from e

def get_script_from_project() -> List[str]:
    script_keys = []
    try:
        script_keys = get_script_keys_from_toml("pyproject.toml")
        print("Found script keys:", script_keys)
    except Exception as e:
        print(f"Error: {str(e)}")
    
    return script_keys

if __name__ == "__main__":
    # for example: uv run mcp-simple-tool --transport sse --port 8000
    launch_command = ""

    script_keys = get_script_from_project()
    if len(script_keys) > 0 :
        launch_command = f"uv run {script_keys[0]} --transport sse --port 8000"
    else: 
        config = get_mcp_cmds_from_json()
        if 'launch_cmds' in config and config['launch_cmds']:
            launch_commands = config['launch_cmds']
            if 'remote' in launch_commands and launch_commands['remote']:
                launch_command = launch_commands['remote']

    print("Starting...")
    exit_code = run_command(cmd=launch_command)
    print(f"Process exited with code {exit_code}")
