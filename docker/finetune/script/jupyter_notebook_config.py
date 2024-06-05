import os

c.ServerApp.ip = '0.0.0.0'
c.ServerApp.token = ""
c.ServerApp.open_browser = False
c.ServerApp.allow_root = True
c.ServerApp.port_retries = 0
c.ServerApp.quit_button = False
c.ServerApp.allow_remote_access = True
c.ServerApp.disable_check_xsrf = True
c.ServerApp.allow_origin = '*'
c.ServerApp.trust_xheaders = True
c.ServerApp.open_browser = False
c.ServerApp.answer_yes = True
c.ServerApp.tornado_settings = {
    "headers": {
        "Content-Security-Policy": "frame-ancestors \'self\' *"
    }
}

# c.ServerApp.base_url = context_path

# opt-in the async version to file handler and checkpoints
c.ServerApp.checkpoints_class = "jupyter_server.services.contents.checkpoints.AsyncCheckpoints"

# Do not delete files to trash: https://github.com/jupyter/notebook/issues/3130
c.FileContentsManager.delete_to_trash = False

c.ContentsManager.allow_hidden = True

# improve the performance of autocompletion, disable Jedi in IPython (the LSP servers for Python use Jedi too)
c.Completer.use_jedi = False

# https://forums.fast.ai/t/jupyter-notebook-enhancements-tips-and-tricks/17064/22
c.NotebookApp.iopub_msg_rate_limit = 100000000
c.NotebookApp.iopub_data_rate_limit = 2147483647

# inject proxy js (it is hack)

# c.ServerProxy['non_service_rewrite_response'] = [proxy_local_server]
c.FileContentsManager.always_delete_dir = True
