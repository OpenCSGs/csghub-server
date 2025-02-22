import multiprocessing
import subprocess
import sys
import os
import requests
import time
import threading

def read_output(stream):
    try:
        for line in iter(stream.readline, b''):
            if line.strip():
                print(line.strip())
    except Exception as e:
        print(f"error: reading output: {e}")

def execute_process(command, env=None):
    if env:
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env)
    else:
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    
    stdout_thread = threading.Thread(target=read_output, args=(process.stdout,))
    stderr_thread = threading.Thread(target=read_output, args=(process.stderr,))
    stdout_thread.start()
    stderr_thread.start()


def run_process_with_env(command):
    env = os.environ.copy()
    env['PYTHONPATH'] = '/workspace'
    execute_process(command, env=env)

def run_process(command):
    execute_process(command, env=None)

def start_vllm_processes(model_name, port):
    vllm_cmd = [
        'python3.10', '-m', 'vllm.entrypoints.openai.api_server', 
        '--trust-remote-code', 
        '--port', port, 
        '--model', model_name
    ]
    vllm_process = multiprocessing.Process(target=run_process_with_env, args=(vllm_cmd,))
    vllm_process.start()
    return vllm_process

def check_vllm_model(port):
    time.sleep(10)
    url = f"http://127.0.0.1:{port}/v1/models"
    while True:
        try:
            response = requests.get(url)
            if response.status_code == 200:
                data = response.json()
                if len(data['data']) > 0:
                    print(f"model inference is ready on {port}")
                    return True
        except Exception as e:
            print(f"warn: model inference is not ready")
        print(f"waiting for model inference is ready on port {port}")
        time.sleep(10)

def start_webui_processes(port):
    ui_cmd = ['open-webui', 'serve', '--port', port]
    ui_process = multiprocessing.Process(target=run_process_with_env, args=(ui_cmd,))
    ui_process.start()
    return ui_process

def start_processes():
    model_name = os.environ.get('MODEL_NAME')
    vllm_port = os.environ.get('API_PORT', '11231')
    webui_port = os.environ.get('WEBUI_PORT', '8080')
    vllm_process = start_vllm_processes(model_name, vllm_port)
    check_vllm_model(vllm_port)
    ui_process = start_webui_processes(webui_port)
    vllm_process.join()
    ui_process.join()

if __name__ == '__main__':
    start_processes()
    print("All processes have finished.")
    