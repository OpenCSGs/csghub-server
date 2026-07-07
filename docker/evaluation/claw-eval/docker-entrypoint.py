#!/usr/bin/env python3
"""Docker entrypoint that maps environment variables to claw-eval CLI args."""

from __future__ import annotations

import os
import sys
from urllib.parse import urlsplit, urlunsplit


MODEL_COMMANDS = {"run", "batch", "_run-inner"}
TASK_TAGS = {"general", "multimodal", "multi_turn"}
PLATFORM_TASK_SETS = {"normal"}
DEFAULT_CONFIG = "config_csghub.yaml"
DEFAULT_TRIALS = "3"
DEFAULT_PARALLEL = "2"
DEFAULT_NORMAL_TASKS_FILE = "/etc/csghub/normal-tasks.json"
DEFAULT_SOURCE_TASKS_DIR = "/app/tasks"

COMMON_VALUE_ARGS = {
    "CLAW_EVAL_CONFIG": "--config",
    "CLAW_EVAL_TRACE_DIR": "--trace-dir",
    "CLAW_EVAL_JUDGE_MODEL": "--judge-model",
    "CLAW_EVAL_PROXY": "--proxy",
}

COMMON_BOOL_ARGS = {
    "CLAW_EVAL_NO_JUDGE": "--no-judge",
}

RUN_VALUE_ARGS = {
    "CLAW_EVAL_TRIALS": "--trials",
    "CLAW_EVAL_PORT_OFFSET": "--port-offset",
}

BATCH_VALUE_ARGS = {
    "CLAW_EVAL_TASKS_DIR": "--tasks-dir",
    "CLAW_EVAL_PARALLEL": "--parallel",
    "CLAW_EVAL_TRIALS": "--trials",
}


def _has_arg(args: list[str], flag: str) -> bool:
    return any(arg == flag or arg.startswith(f"{flag}=") for arg in args)


def _arg_value(args: list[str], flag: str) -> str | None:
    for i, arg in enumerate(args):
        if arg.startswith(f"{flag}="):
            return arg.split("=", 1)[1]
        if arg == flag and i + 1 < len(args):
            return args[i + 1]
    return None


def _first_env(names: list[str]) -> tuple[str | None, str | None]:
    for name in names:
        value = os.environ.get(name)
        if value:
            return name, value
    return None, None


def _has_runtime_env() -> bool:
    names = {
        "CLAW_EVAL_COMMAND",
        "CLAW_EVAL_TASKS",
        "CLAW_EVAL_MODEL",
        "MODEL_ID",
        "MODEL",
        "CLAW_EVAL_API_KEY",
        "OPENAI_API_KEY",
        "CLAW_EVAL_BASE_URL",
        "OPENAI_BASE_URL",
    }
    return any(os.environ.get(name) for name in names)


def _env_truthy(name: str) -> bool:
    value = os.environ.get(name)
    if not value:
        return False
    return value.lower() not in {"0", "false", "no", "off"}


def _append_value_args(args: list[str], mapping: dict[str, str]) -> None:
    for env_name, flag in mapping.items():
        value = os.environ.get(env_name)
        if value and not _has_arg(args, flag):
            args.extend([flag, value])


def _append_default_value_arg(args: list[str], flag: str, value: str) -> None:
    if not _has_arg(args, flag):
        args.extend([flag, value])


def _append_bool_args(args: list[str], mapping: dict[str, str]) -> None:
    for env_name, flag in mapping.items():
        if _env_truthy(env_name) and not _has_arg(args, flag):
            args.append(flag)


def _append_default_sandbox_tools(args: list[str]) -> None:
    if _has_arg(args, "--sandbox") or _has_arg(args, "--sandbox-tools"):
        return
    if os.environ.get("CLAW_EVAL_DOCKER_SANDBOX_TOOLS", "1").lower() in {"0", "false", "no", "off"}:
        return
    args.append("--sandbox-tools")


def _append_default_judge_model(args: list[str]) -> None:
    if _has_arg(args, "--judge-model") or _has_arg(args, "--no-judge"):
        return
    if _env_truthy("CLAW_EVAL_NO_JUDGE"):
        args.append("--no-judge")
        return
    _, judge_model = _first_env(["CLAW_EVAL_JUDGE_MODEL"])
    if judge_model:
        args.extend(["--judge-model", judge_model])


def _prepare_normal_tasks_dir() -> str:
    import json
    import tempfile
    from pathlib import Path

    tasks_file = os.environ.get("CLAW_EVAL_NORMAL_TASKS_FILE", DEFAULT_NORMAL_TASKS_FILE)
    with open(tasks_file, encoding="utf-8") as handle:
        payload = json.load(handle)
    task_names = payload.get("tasks") or []
    if not task_names:
        raise SystemExit(f"No tasks listed in {tasks_file}")

    src_root = Path(os.environ.get("CLAW_EVAL_SOURCE_TASKS_DIR", DEFAULT_SOURCE_TASKS_DIR))
    app_root = src_root.parent
    work_root = Path(tempfile.mkdtemp(prefix="claw-normal-tasks-"))
    tasks_dir = work_root / "tasks"
    tasks_dir.mkdir()
    mock_services = app_root / "mock_services"
    if mock_services.is_dir():
        os.symlink(mock_services, work_root / "mock_services")
    linked = 0
    for name in task_names:
        src = src_root / name
        dst = tasks_dir / name
        if src.is_dir():
            os.symlink(src, dst)
            linked += 1
    if linked == 0:
        raise SystemExit(f"No normal tasks found under {src_root}")
    print(f"info: prepared {linked} normal tasks in {tasks_dir}", file=sys.stderr)
    return str(tasks_dir)


def _append_task_selector(args: list[str]) -> None:
    selector = os.environ.get("CLAW_EVAL_TASKS", "normal").strip()
    if not selector or selector.lower() in {"all"}:
        return

    if selector.lower() == "normal":
        if not _has_arg(args, "--tasks-dir"):
            args.extend(["--tasks-dir", _prepare_normal_tasks_dir()])
        return

    selector_tags = {part.strip() for part in selector.split(",") if part.strip()}
    if selector_tags and selector_tags.issubset(TASK_TAGS):
        if not _has_arg(args, "--tag"):
            args.extend(["--tag", selector])
        return

    if "-" in selector and all(part.isdigit() for part in selector.split("-", 1)):
        if not _has_arg(args, "--range"):
            args.extend(["--range", selector])
        return

    if not _has_arg(args, "--filter"):
        args.extend(["--filter", selector])


def _apply_env_args(args: list[str]) -> None:
    if not args:
        return

    command = args[0]
    _append_value_args(args, COMMON_VALUE_ARGS)
    _append_bool_args(args, COMMON_BOOL_ARGS)
    _append_default_value_arg(args, "--config", os.environ.get("CLAW_EVAL_CONFIG", DEFAULT_CONFIG))

    if command == "batch":
        _append_value_args(args, BATCH_VALUE_ARGS)
        _append_default_value_arg(args, "--trials", DEFAULT_TRIALS)
        _append_default_value_arg(args, "--parallel", DEFAULT_PARALLEL)
        _append_task_selector(args)
        _append_default_sandbox_tools(args)
    elif command == "run":
        _append_value_args(args, RUN_VALUE_ARGS)
        _append_default_value_arg(args, "--trials", DEFAULT_TRIALS)
        _append_default_sandbox_tools(args)


def _rewrite_localhost_url(url: str) -> str:
    if os.environ.get("CLAW_EVAL_DOCKER_REWRITE_LOCALHOST", "1").lower() in {"0", "false", "no"}:
        return url

    parsed = urlsplit(url)
    if parsed.hostname not in {"localhost", "127.0.0.1"}:
        return url

    host = "host.docker.internal"
    if parsed.port:
        host = f"{host}:{parsed.port}"
    if parsed.username or parsed.password:
        auth = parsed.username or ""
        if parsed.password:
            auth = f"{auth}:{parsed.password}"
        host = f"{auth}@{host}"

    return urlunsplit((parsed.scheme, host, parsed.path, parsed.query, parsed.fragment))


def main() -> None:
    args = sys.argv[1:]
    os.environ.setdefault("CLAW_EVAL_EMIT_RESULTS", "1")

    env_command = os.environ.get("CLAW_EVAL_COMMAND")
    if env_command and (not args or args == ["--help"]):
        args = [env_command]
    elif not args or (args == ["--help"] and _has_runtime_env()):
        args = ["batch"]

    if args and args[0] in MODEL_COMMANDS:
        key_name, api_key = _first_env([
            "CLAW_EVAL_API_KEY",
            "OPENAI_API_KEY",
            "OPENAI_STYLE_API_KEY",
            "ANTHROPIC_API_KEY",
        ])
        url_name, base_url = _first_env([
            "CLAW_EVAL_BASE_URL",
            "OPENAI_BASE_URL",
            "OPENAI_STYLE_BASE_URL",
            "ANTHROPIC_BASE_URL",
        ])
        _, model = _first_env(["CLAW_EVAL_MODEL", "MODEL_ID", "MODEL"])

        if base_url:
            rewritten_base_url = _rewrite_localhost_url(base_url)
            if rewritten_base_url != base_url:
                print(
                    f"info: rewriting model base URL for Docker: {base_url} -> {rewritten_base_url}",
                    file=sys.stderr,
                )
                base_url = rewritten_base_url

        if api_key:
            os.environ.setdefault("OPENROUTER_API_KEY", api_key)
        if base_url:
            os.environ.setdefault("OPENAI_BASE_URL", base_url)

        judge_base_url = os.environ.get("CLAW_EVAL_JUDGE_BASE_URL")
        if judge_base_url:
            judge_base_url = _rewrite_localhost_url(judge_base_url)
            os.environ["CLAW_EVAL_JUDGE_BASE_URL"] = judge_base_url

        if api_key and not _has_arg(args, "--api-key"):
            args.extend(["--api-key", api_key])
        if base_url and not _has_arg(args, "--base-url"):
            args.extend(["--base-url", base_url])
        if model and not _has_arg(args, "--model"):
            args.extend(["--model", model])

        _apply_env_args(args)
        _append_default_judge_model(args)

        if url_name == "ANTHROPIC_BASE_URL" and base_url and "api.anthropic.com" in base_url:
            print(
                "warning: claw-eval currently calls OpenAI-compatible chat completions; "
                "native Anthropic /v1/messages URLs are not supported by this Docker wrapper.",
                file=sys.stderr,
            )
        elif key_name == "ANTHROPIC_API_KEY" and not base_url:
            print(
                "warning: ANTHROPIC_API_KEY was provided without an OpenAI-compatible "
                "ANTHROPIC_BASE_URL/CLAW_EVAL_BASE_URL.",
                file=sys.stderr,
            )

    os.execvp("claw-eval", ["claw-eval", *args])


if __name__ == "__main__":
    main()
