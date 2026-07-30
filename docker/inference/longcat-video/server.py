"""Asynchronous LightX2V-compatible API for LongCat Video Avatar models."""

from __future__ import annotations

import json
import logging
import os
import queue
import shutil
import subprocess
import tempfile
import threading
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import quote, urlparse

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import FileResponse


def resolve_model_type(repo_id: str) -> str:
    model_name = Path(repo_id).name
    if model_name == "LongCat-Video-Avatar":
        return "avatar-v1.0"
    if model_name == "LongCat-Video-Avatar-1.5":
        return "avatar-v1.5"
    raise ValueError(f"unsupported LongCat Avatar repository: {repo_id}")


LOGGER = logging.getLogger("longcat-video")

WORKSPACE = Path(os.getenv("WORKSPACE", "/workspace"))
CODE_DIR = Path("/opt/LongCat-Video")
REPO_ID = os.environ["REPO_ID"]
MODEL_DIR = WORKSPACE / REPO_ID
MODEL_TYPE = resolve_model_type(REPO_ID)
OUTPUT_DIR = WORKSPACE / "outputs" / "videos"
TASK_NAMESPACE = os.getenv("LONGCAT_TASK_NAMESPACE", REPO_ID).strip() or REPO_ID
TASK_STATE_DIR = WORKSPACE / "outputs" / "video-tasks" / quote(TASK_NAMESPACE, safe="")
GPU_NUM = int(os.getenv("GPU_NUM", "1"))
MAX_FILE_BYTES = int(os.getenv("MAX_FILE_BYTES", str(100 * 1024 * 1024)))
MAX_PROMPT_LENGTH = int(os.getenv("MAX_PROMPT_LENGTH", "4096"))
S3_ACCESS_ID = os.getenv("S3_ACCESS_ID", "").strip()
S3_ACCESS_SECRET = os.getenv("S3_ACCESS_SECRET", "").strip()
S3_BUCKET = os.getenv("S3_BUCKET", "").strip()
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "").strip()
S3_SSL_ENABLED = os.getenv("S3_SSL_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}
if GPU_NUM < 1:
    raise ValueError("GPU_NUM must be at least 1")

ALLOWED_AUDIO_TYPES = {"para", "add"}
ALLOWED_RESOLUTIONS = {
    "480p": (832, 480),
    "720p": (1280, 768),
}
ALLOWED_IMAGE_MIMES = {"image/jpeg", "image/png", "image/webp"}


@dataclass
class Task:
    task_id: str
    work_dir: Path
    input_json: Path
    output_dir: Path
    audio_count: int
    resolution: str
    num_segments: int
    ref_img_index: int
    mask_frame_range: int
    status: str = "submitted"
    error: str | None = None
    command: list[str] = field(default_factory=list)
    object_key: str | None = None
    download_url: str | None = None


tasks: dict[str, Task] = {}
tasks_lock = threading.Lock()
task_queue: queue.Queue[Task] = queue.Queue()
worker_thread: threading.Thread | None = None

app = FastAPI(title="LongCat Video Avatar")


def _s3_upload_enabled() -> bool:
    return bool(S3_BUCKET and S3_ENDPOINT and S3_ACCESS_ID and S3_ACCESS_SECRET)


def _task_state(task: Task) -> dict[str, Any]:
    state: dict[str, Any] = {
        "id": task.task_id,
        "task_id": task.task_id,
        "status": task.status,
    }
    if task.error:
        state["error"] = task.error
        state["message"] = task.error
    if task.object_key:
        state["object_key"] = task.object_key
    if task.download_url:
        state["download_url"] = task.download_url
    return state


def _state_path(task_id: str) -> Path:
    try:
        normalized = uuid.UUID(hex=task_id).hex
    except ValueError as exc:
        raise HTTPException(404, "task not found") from exc
    if normalized != task_id:
        raise HTTPException(404, "task not found")
    return TASK_STATE_DIR / f"{normalized}.json"


def _write_task_state(state: dict[str, Any]) -> None:
    task_id = str(state.get("task_id", ""))
    path = _state_path(task_id)
    TASK_STATE_DIR.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(prefix=f".{task_id}-", suffix=".tmp", dir=TASK_STATE_DIR)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            json.dump(state, output, ensure_ascii=False)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temp_name, path)
    except Exception:
        Path(temp_name).unlink(missing_ok=True)
        raise


def _persist_task(task: Task) -> None:
    _write_task_state(_task_state(task))


def _read_task_state(task_id: str) -> dict[str, Any]:
    path = _state_path(task_id)
    try:
        state = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise HTTPException(404, "task not found") from exc
    if not isinstance(state, dict) or state.get("task_id") != task_id:
        raise HTTPException(404, "task not found")
    return state


def _recover_task_states() -> None:
    TASK_STATE_DIR.mkdir(parents=True, exist_ok=True)
    for path in TASK_STATE_DIR.glob("*.json"):
        task_id = path.stem
        try:
            state = _read_task_state(task_id)
        except HTTPException:
            LOGGER.warning("ignoring invalid task state file %s", path)
            continue
        if state.get("status") not in {"submitted", "running"}:
            continue
        message = "task interrupted by service restart; submit a new generation request"
        state["status"] = "failed"
        state["error"] = message
        state["message"] = message
        _write_task_state(state)


def _s3_endpoint_host() -> str:
    parsed = urlparse(S3_ENDPOINT if "://" in S3_ENDPOINT else f"http://{S3_ENDPOINT}")
    return parsed.netloc or parsed.path


def _object_download_url(object_key: str) -> str:
    endpoint_host = _s3_endpoint_host()
    encoded_key = quote(object_key, safe="/")
    scheme = "https" if S3_SSL_ENABLED else "http"
    if "aliyuncs.com" in endpoint_host:
        return f"{scheme}://{S3_BUCKET}.{endpoint_host}/{encoded_key}"
    return f"{scheme}://{endpoint_host}/{quote(S3_BUCKET, safe='')}/{encoded_key}"


def _upload_video(path: Path, task_id: str) -> tuple[str, str]:
    object_key = f"aigateway/generated/videos/{task_id}.mp4"
    endpoint_host = _s3_endpoint_host()
    if "aliyuncs.com" in endpoint_host:
        import oss2

        endpoint = f"{'https' if S3_SSL_ENABLED else 'http'}://{endpoint_host}"
        bucket = oss2.Bucket(oss2.Auth(S3_ACCESS_ID, S3_ACCESS_SECRET), endpoint, S3_BUCKET)
        bucket.put_object_from_file(object_key, str(path))
    else:
        from minio import Minio

        client = Minio(
            endpoint_host,
            access_key=S3_ACCESS_ID,
            secret_key=S3_ACCESS_SECRET,
            secure=S3_SSL_ENABLED,
        )
        client.fput_object(S3_BUCKET, object_key, str(path), content_type="video/mp4")
    return object_key, _object_download_url(object_key)


def _field(form: Any, name: str, default: Any = None) -> Any:
    value = form.get(name)
    return default if value is None or value == "" else value


def _parse_int(form: Any, name: str, default: int, minimum: int, maximum: int) -> int:
    try:
        value = int(_field(form, name, default))
    except (TypeError, ValueError) as exc:
        raise HTTPException(400, f"{name} must be an integer") from exc
    if not minimum <= value <= maximum:
        raise HTTPException(400, f"{name} must be between {minimum} and {maximum}")
    return value


def _parse_resolution(form: Any) -> str:
    resolution = _field(form, "resolution")
    width_raw, height_raw = _field(form, "width"), _field(form, "height")
    if (width_raw is None) != (height_raw is None):
        raise HTTPException(400, "width and height must be provided together")
    if width_raw is not None:
        try:
            dimensions = (int(width_raw), int(height_raw))
        except (TypeError, ValueError) as exc:
            raise HTTPException(400, "width and height must be integers") from exc
        matches = [name for name, size in ALLOWED_RESOLUTIONS.items() if size == dimensions]
        if not matches:
            raise HTTPException(400, "supported dimensions are 832x480 and 1280x768")
        if resolution and resolution != matches[0]:
            raise HTTPException(400, "resolution conflicts with width and height")
        resolution = matches[0]
    resolution = resolution or "480p"
    if resolution not in ALLOWED_RESOLUTIONS:
        raise HTTPException(400, "resolution must be 480p or 720p")
    return resolution


def _parse_bbox(raw: Any) -> dict[str, list[int]] | None:
    if raw is None or raw == "":
        return None
    try:
        bbox = json.loads(raw)
    except (TypeError, json.JSONDecodeError) as exc:
        raise HTTPException(400, "bbox must be valid JSON") from exc
    if not isinstance(bbox, dict) or not set(bbox).issubset({"person1", "person2", "others"}):
        raise HTTPException(400, "bbox must be an object containing person1, person2, or others")
    for key, value in bbox.items():
        if (
            not isinstance(value, list)
            or not value
            or len(value) % 4
            or any(not isinstance(item, int) or isinstance(item, bool) or item < 0 for item in value)
        ):
            raise HTTPException(400, f"bbox.{key} must contain non-negative integer groups of four")
        if key != "others" and len(value) != 4:
            raise HTTPException(400, f"bbox.{key} must contain exactly four integers")
    if ("person1" in bbox) != ("person2" in bbox):
        raise HTTPException(400, "bbox person1 and person2 must be provided together")
    return bbox


async def _save_upload(upload: Any, destination: Path, allowed_mimes: set[str] | None) -> None:
    content_type = (upload.content_type or "").lower()
    if allowed_mimes is None:
        if not content_type.startswith("audio/"):
            raise HTTPException(415, "audio files must use an audio/* MIME type")
    elif content_type not in allowed_mimes:
        raise HTTPException(415, f"unsupported file MIME type: {content_type or 'missing'}")

    total = 0
    with destination.open("xb") as output:
        while chunk := await upload.read(1024 * 1024):
            total += len(chunk)
            if total > MAX_FILE_BYTES:
                output.close()
                destination.unlink(missing_ok=True)
                raise HTTPException(413, f"each upload must be at most {MAX_FILE_BYTES} bytes")
            output.write(chunk)
    if total == 0:
        destination.unlink(missing_ok=True)
        raise HTTPException(400, "uploaded files cannot be empty")


def build_command(task: Task) -> list[str]:
    multi = task.audio_count == 2
    script = (
        "run_demo_avatar_multi_audio_to_video.py"
        if multi
        else "run_demo_avatar_single_audio_to_video.py"
    )
    command = [
        "torchrun",
        f"--nproc_per_node={GPU_NUM}",
        str(CODE_DIR / script),
        f"--context_parallel_size={GPU_NUM}",
        f"--checkpoint_dir={MODEL_DIR}",
        f"--input_json={task.input_json}",
        f"--output_dir={task.output_dir}",
        f"--resolution={task.resolution}",
        f"--num_segments={task.num_segments}",
        f"--ref_img_index={task.ref_img_index}",
        f"--mask_frame_range={task.mask_frame_range}",
        f"--model_type={MODEL_TYPE}",
    ]
    if MODEL_TYPE == "avatar-v1.5":
        command.extend(["--use_distill", "--use_int8"])
    if not multi:
        with task.input_json.open(encoding="utf-8") as input_file:
            stage = "ai2v" if "cond_image" in json.load(input_file) else "at2v"
        command.append(f"--stage_1={stage}")
    return command


def run_task(task: Task) -> None:
    try:
        with tasks_lock:
            task.status = "running"
        _persist_task(task)
        task.command = build_command(task)
        # The official scripts create relative scratch directories. Running
        # inside the per-task directory keeps every temporary artifact scoped.
        subprocess.run(task.command, cwd=task.work_dir, check=True)
        videos = list(task.output_dir.rglob("*.mp4"))
        if not videos:
            raise RuntimeError("LongCat inference produced no MP4 file")
        generated = max(videos, key=lambda path: path.stat().st_mtime_ns)
        OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
        final_path = OUTPUT_DIR / f"{task.task_id}.mp4"
        shutil.move(str(generated), final_path)
        if _s3_upload_enabled():
            try:
                task.object_key, task.download_url = _upload_video(final_path, task.task_id)
            except Exception:
                LOGGER.exception(
                    "failed to upload video for task %s; local download remains available",
                    task.task_id,
                )
        with tasks_lock:
            task.status = "succeed"
        _persist_task(task)
    except Exception as exc:
        LOGGER.exception("task %s failed", task.task_id)
        with tasks_lock:
            task.status = "failed"
            task.error = str(exc)
        try:
            _persist_task(task)
        except Exception:
            LOGGER.exception("failed to persist failure state for task %s", task.task_id)
    finally:
        shutil.rmtree(task.work_dir, ignore_errors=True)


def _worker() -> None:
    while True:
        run_task(task_queue.get())
        task_queue.task_done()


@app.on_event("startup")
def startup() -> None:
    global worker_thread
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    _recover_task_states()
    if S3_BUCKET and not _s3_upload_enabled():
        LOGGER.warning("S3_BUCKET is set but S3 upload configuration is incomplete; local video storage remains enabled")
    if worker_thread is None or not worker_thread.is_alive():
        worker_thread = threading.Thread(target=_worker, name="longcat-worker", daemon=True)
        worker_thread.start()


@app.post("/v1/tasks/video/form", status_code=202)
async def submit_task(request: Request) -> dict[str, str]:
    form = await request.form()
    prompt = str(_field(form, "prompt", "")).strip()
    if not prompt or len(prompt) > MAX_PROMPT_LENGTH:
        raise HTTPException(400, f"prompt must contain 1-{MAX_PROMPT_LENGTH} characters")

    uploads = [
        (key, item) for key, item in form.multi_items() if hasattr(item, "read")
    ]
    unknown_uploads = [key for key, _ in uploads if key not in {"audio", "audio_file", "image_file"}]
    if unknown_uploads:
        raise HTTPException(400, f"unsupported file field: {unknown_uploads[0]}")
    audio_uploads = [
        item
        for key, item in uploads
        if key in {"audio", "audio_file"}
    ]
    image_uploads = [
        item for key, item in uploads if key == "image_file"
    ]
    if not 1 <= len(audio_uploads) <= 2:
        raise HTTPException(400, "exactly one or two audio files are required")
    if len(image_uploads) > 1:
        raise HTTPException(400, "at most one image_file is allowed")
    if len(audio_uploads) == 2 and not image_uploads:
        raise HTTPException(400, "two-audio generation requires image_file")

    audio_type = str(_field(form, "audio_type", "para"))
    if audio_type not in ALLOWED_AUDIO_TYPES:
        raise HTTPException(400, "audio_type must be para or add")
    resolution = _parse_resolution(form)
    num_segments = _parse_int(form, "num_segments", 1, 1, 100)
    ref_img_index = _parse_int(form, "ref_img_index", 10, -1000, 1000)
    mask_frame_range = _parse_int(form, "mask_frame_range", 3, 0, 1000)
    bbox = _parse_bbox(_field(form, "bbox"))
    if bbox and len(audio_uploads) != 2:
        raise HTTPException(400, "bbox is only supported for two-audio generation")

    task_id = uuid.uuid4().hex
    work_dir = Path(tempfile.mkdtemp(prefix=f"longcat-{task_id}-"))
    request_dir, generated_dir = work_dir / "request", work_dir / "generated"
    request_dir.mkdir()
    generated_dir.mkdir()
    try:
        audio_paths = []
        for index, upload in enumerate(audio_uploads, 1):
            suffix = Path(upload.filename or "").suffix.lower()[:10] or ".audio"
            path = request_dir / f"audio{index}{suffix}"
            await _save_upload(upload, path, None)
            audio_paths.append(path)
        image_path = None
        if image_uploads:
            suffix = Path(image_uploads[0].filename or "").suffix.lower()[:10] or ".image"
            image_path = request_dir / f"image{suffix}"
            await _save_upload(image_uploads[0], image_path, ALLOWED_IMAGE_MIMES)

        payload: dict[str, Any] = {
            "prompt": prompt,
            "cond_audio": {
                f"person{index}": str(path) for index, path in enumerate(audio_paths, 1)
            },
        }
        if image_path:
            payload["cond_image"] = str(image_path)
        if len(audio_paths) == 2:
            payload["audio_type"] = audio_type
            if bbox:
                payload["bbox"] = bbox
        input_json = request_dir / "input.json"
        input_json.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
    except Exception:
        shutil.rmtree(work_dir, ignore_errors=True)
        raise

    task = Task(
        task_id=task_id,
        work_dir=work_dir,
        input_json=input_json,
        output_dir=generated_dir,
        audio_count=len(audio_paths),
        resolution=resolution,
        num_segments=num_segments,
        ref_img_index=ref_img_index,
        mask_frame_range=mask_frame_range,
    )
    with tasks_lock:
        tasks[task_id] = task
    try:
        _persist_task(task)
    except Exception:
        with tasks_lock:
            tasks.pop(task_id, None)
        shutil.rmtree(work_dir, ignore_errors=True)
        LOGGER.exception("failed to persist submitted task %s", task_id)
        raise HTTPException(500, "failed to persist task") from None
    task_queue.put(task)
    return {"id": task_id, "task_id": task_id, "status": "submitted"}


@app.get("/v1/tasks/{task_id}/status")
def task_status(task_id: str) -> dict[str, Any]:
    state = _read_task_state(task_id)
    response: dict[str, Any] = {
        "id": task_id,
        "task_id": task_id,
        "status": state.get("status", ""),
    }
    if state.get("error"):
        response["error"] = state["error"]
        response["message"] = state.get("message", state["error"])
    if state.get("download_url"):
        response["download_url"] = state["download_url"]
    if state.get("status") == "succeed":
        response["output"] = f"/v1/files/download/outputs/videos/{task_id}.mp4"
    return response


@app.get("/v1/files/download/outputs/videos/{task_id}.mp4")
def download_video(task_id: str) -> FileResponse:
    try:
        uuid.UUID(hex=task_id)
    except ValueError as exc:
        raise HTTPException(404, "file not found") from exc
    path = OUTPUT_DIR / f"{task_id}.mp4"
    if not path.is_file():
        raise HTTPException(404, "file not found")
    return FileResponse(path, media_type="video/mp4", filename=f"{task_id}.mp4")


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok",
        "queue_size": task_queue.qsize(),
        "worker_alive": bool(worker_thread and worker_thread.is_alive()),
    }
