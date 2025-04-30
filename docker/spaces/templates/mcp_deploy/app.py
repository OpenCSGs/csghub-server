import subprocess
import os
import sys
import json
import tomli
import select
import tempfile
import re
from typing import Union, Any, Dict, List, Tuple
from pathlib import Path

KEY_LANGUAGE = "program_language"
KEY_RUNMODE = "run_mode"
KEY_INSTALL = "install_deps_cmds"
KEY_BUILD = "build_cmds"
KEY_LAUNCH = "launch_cmds"
KEY_NODE = "node"

def init_config() -> Dict[str, Any]: 
    config = dict()
    config[KEY_LANGUAGE] = ""
    config[KEY_RUNMODE] = ""
    config[KEY_INSTALL] = ""
    config[KEY_BUILD] = ""
    config[KEY_LAUNCH] = ""
    config[KEY_NODE] = ""
    return config

def run_command_in_process(cmd: Union[str, List[str]], env: dict[str, str]) -> int:
    try:
        with subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
            bufsize=1,
            env=env,
        ) as process:
            while True:
                reads = [process.stdout, process.stderr]
                ret = select.select(reads, [], [])
                
                for stream in ret[0]:
                    if stream == process.stdout:
                        line = process.stdout.readline()
                        if line:
                            print(line, end='')
                    elif stream == process.stderr:
                        line = process.stderr.readline()
                        if line:
                            print(line, end='', file=sys.stderr)
                
                if process.poll() is not None:
                    for line in process.stdout:
                        print(line, end='')
                    for line in process.stderr:
                        print(line, end='', file=sys.stderr)
                    break
                    
            return process.returncode
            
    except FileNotFoundError:
        print(f"Error: Command not found - {' '.join(cmd)}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Execution failed: {str(e)}", file=sys.stderr)
        return 1

def run_python_project(cmd: Union[str, List[str]]) -> int:
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

    exit_code = run_command_in_process(cmd=cmd, env=env)
    return exit_code

def run_typescript_project(
    install_cmd: Union[str, List[str]],
    build_cmd: Union[str, List[str]], 
    start_cmd: Union[str, List[str]],
    node: str,
    ) -> int:
    env = os.environ.copy()
    home = env['HOME']
    node = node.strip()
    if not node:
        node = "18"

    temp_file_path = ""
    with tempfile.NamedTemporaryFile(mode='w+', suffix='.sh', delete=False) as temp_file:
        temp_file.write(f"""#!/bin/bash
source {home}/.nvm/nvm.sh
nvm install {node}
{install_cmd}
{build_cmd}
{start_cmd}
""")
        temp_file_path = temp_file.name
        
    os.chmod(temp_file_path, 0o755)
    run_cmd = f"bash {temp_file_path}"
    exit_code = run_command_in_process(cmd=run_cmd.split(), env=env)
    if exit_code != 0:
        print(f"Error: Failed to run command - {run_cmd}")
        
    return exit_code

def run_mcp_project(
    install_cmd: Union[str, List[str]],
    build_cmd: Union[str, List[str]], 
    start_cmd: Union[str, List[str]],
    ) -> int:
    env = os.environ.copy()

    temp_file_path = ""
    with tempfile.NamedTemporaryFile(mode='w+', suffix='.sh', delete=False) as temp_file:
        temp_file.write(f"""#!/bin/bash
{install_cmd}
{build_cmd}
{start_cmd}
""")
        temp_file_path = temp_file.name
        
    os.chmod(temp_file_path, 0o755)
    run_cmd = f"bash {temp_file_path}"
    exit_code = run_command_in_process(cmd=run_cmd.split(), env=env)
    if exit_code != 0:
        print(f"Error: Failed to run command - {run_cmd}")
        
    return exit_code

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

def get_mcp_cmds_from_config() -> Tuple[Dict[str, Any], bool]:
    config = init_config()
    is_ok = False
    try:
        jsonData = read_json_file("mcp_space_conf.json")
        print("Successfully loaded space JSON config:")
        print(json.dumps(jsonData, indent=2, ensure_ascii=False))
        if KEY_INSTALL in jsonData and jsonData[KEY_INSTALL]:
            config[KEY_INSTALL]= jsonData[KEY_INSTALL]
        if KEY_BUILD in jsonData and jsonData[KEY_BUILD]:
            config[KEY_BUILD] = jsonData[KEY_BUILD]
        if KEY_LAUNCH in jsonData and jsonData[KEY_LAUNCH]:
            config[KEY_LAUNCH]= jsonData[KEY_LAUNCH]
            is_ok = True
    except Exception as e:
        print(f"Warning: Invalid MCP space configuration (mcp_space_conf.json) - {str(e)}")

    return config, is_ok

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

def get_script_from_python_project() -> Tuple[Dict[str, Any], bool]:
    config = init_config()
    is_python = False
    try:
        # python example: uv run mcp-simple-tool --transport sse --port 8000
        script_keys = get_script_keys_from_toml("pyproject.toml")
        if len(script_keys) > 0:
            print("Found python project script keys: ", script_keys)
            config[KEY_LAUNCH]= f"uv run {script_keys[0]} --transport sse --port 8000"
            is_python = True
    except Exception as e:
        print(f"Warning: No valid Python project detected - {str(e)}")
    
    return config, is_python

def extract_node_version(version_str: str) -> str:
    version_str = version_str.strip()
    match = re.search(r'[\d.]+', version_str)
    if match:
        return match.group()
    return ""

def get_script_from_ts_project() -> Tuple[Dict[str, Any], bool]:
    config = init_config()
    is_ok = False
    try:
        jsonData = read_json_file("package.json")
        print("Successfully loaded TS package JSON config")
        # print(json.dumps(jsonData, indent=2, ensure_ascii=False))
        if 'engines' in jsonData and jsonData['engines']:
            ts_engines = jsonData['engines']
            if KEY_NODE in ts_engines and ts_engines[KEY_NODE]:
                config[KEY_NODE] = extract_node_version(ts_engines[KEY_NODE])
        if 'scripts' in jsonData and jsonData['scripts']:
            ts_scripts = jsonData['scripts']
            print("Found TS project scripts: ", ts_scripts)
            config[KEY_INSTALL] = "npm install"
            if 'build' in ts_scripts and ts_scripts['build']:
                config[KEY_BUILD]= "npm run build"
            if 'start' in ts_scripts and ts_scripts['start']:
                config[KEY_LAUNCH]= "npm run start"
                is_ok = True
    except Exception as e:
        print(f"Warning: Invalid TS project detected - {str(e)}")

    return config, is_ok

def run_standard_python_space(config):
    print(f"Python config: {config}")
    launch_command = config[KEY_LAUNCH]
    if launch_command:
        print("Starting Python MCP server...")
        exit_code = run_python_project(cmd=launch_command)
        print(f"Process exited with code {exit_code}")
    else:
        print("Error: No any valid python launch command")

def run_standard_ts_space(config):
    print(f"TS config: {config}")
    install_cmd = config[KEY_INSTALL]
    build_command = config[KEY_BUILD]
    launch_command = config[KEY_LAUNCH]
    node = config[KEY_NODE]
    if launch_command:
        print("Starting TS MCP server...")
        exit_code = run_typescript_project(
            install_cmd=install_cmd,
            build_cmd=build_command,
            start_cmd=launch_command,
            node=node,
        )
        print(f"Process exited with code {exit_code}")
    else:
        print("Error: No any valid TS launch command")   

def run_mcp_space(config):
    print(f"MCP config: {config}")
    install_cmd = config[KEY_INSTALL]
    build_command = config[KEY_BUILD]
    launch_command = config[KEY_LAUNCH]
    if launch_command:
        print("Starting MCP server...")
        exit_code = run_mcp_project(
            install_cmd=install_cmd,
            build_cmd=build_command,
            start_cmd=launch_command,
        )
        print(f"Process exited with code {exit_code}")
    else:
        print("Error: No any valid launch command")

def run_space():
    config, ok = get_script_from_python_project()
    if ok:
        run_standard_python_space(config)
        return
    
    config, ok = get_script_from_ts_project()
    if ok:
        run_standard_ts_space(config)
        return
    
    config, ok = get_mcp_cmds_from_config()
    if ok:
        run_mcp_space(config)
    else:
        print("Error: Our system encountered an issue processing your request. For resolution, please retry or consult our support team.")

if __name__ == "__main__":
    run_space()

