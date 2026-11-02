"""Launch SGLang Diffusion with the LightX2V video routes on the same port.

The inference playground calls POST /v1/tasks/video, GET /v1/tasks/{id}/status
and GET /v1/files/download/outputs/videos/{id}.mp4, as LightX2V and LongCat
serve them. SGLang Diffusion serves POST /v1/videos instead, and MiniMax-H3
also requires task and target. The middleware translates the LightX2V routes
onto the SGLang ones inside the SGLang HTTP process.
"""

import json
import math
import os
import sys
import tempfile
import time
from urllib.parse import parse_qsl

SUPPORTED_TASKS = {"t2va", "fl2va", "ref2va"}
ASPECT_RATIOS = (
    ("21:9", 21 / 9),
    ("16:9", 16 / 9),
    ("4:3", 4 / 3),
    ("1:1", 1),
    ("3:4", 3 / 4),
    ("9:16", 9 / 16),
)
DEFAULT_SHORT_EDGE = 768
DEFAULT_ASPECT_RATIO = "16:9"
DEFAULT_DURATION_SECONDS = 5
UPLOAD_DIR = os.environ.get("SGLANG_VIDEO_COMPAT_UPLOAD_DIR", "/tmp/csghub-video-compat")
UPLOAD_MAX_AGE_SECONDS = 24 * 60 * 60
CREATE_PATHS = {"/v1/tasks/video", "/v1/tasks/video/form"}
DOWNLOAD_PREFIX = "/v1/files/download/"


def nearest_aspect_ratio(width, height):
    ratio = width / height
    best_name, best_ratio = ASPECT_RATIOS[0]
    for name, candidate in ASPECT_RATIOS[1:]:
        if abs(candidate - ratio) < abs(best_ratio - ratio):
            best_name, best_ratio = name, candidate
    return best_name


def finite_number(value):
    if isinstance(value, bool) or value is None or value == "":
        return None
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    if not math.isfinite(number):
        return None
    return number


def whole_number(value):
    number = finite_number(value)
    if number is None or abs(number - int(number)) > 1e-9:
        return None
    return int(number)


def nonempty(value):
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def dimensions(fields):
    width = whole_number(fields.get("width"))
    height = whole_number(fields.get("height"))
    if width and height:
        return width, height
    size = fields.get("size")
    if isinstance(size, str) and "x" in size.lower():
        left, right = size.lower().split("x", 1)
        return whole_number(left), whole_number(right)
    return None, None


def duration_seconds(fields):
    for key in ("video_duration", "seconds"):
        number = finite_number(fields.get(key))
        if number is not None and number > 0:
            return int(number) if number.is_integer() else number
    return DEFAULT_DURATION_SECONDS


def build_target(fields):
    supplied = fields.get("target")
    if isinstance(supplied, str):
        try:
            supplied = json.loads(supplied)
        except json.JSONDecodeError:
            supplied = None
    if isinstance(supplied, dict):
        return supplied
    width, height = dimensions(fields)
    target = {
        "short_edge": DEFAULT_SHORT_EDGE,
        "aspect_ratio": DEFAULT_ASPECT_RATIO,
        "duration_seconds": duration_seconds(fields),
    }
    if width and height:
        target["short_edge"] = min(width, height)
        target["aspect_ratio"] = nearest_aspect_ratio(width, height)
    return target


def lightx2v_to_sglang(fields, image_path=None):
    """Map a LightX2V create body onto a MiniMax-H3 /v1/videos body.

    negative_prompt is dropped because MiniMax-H3 rejects it.
    """
    prompt = nonempty(fields.get("prompt"))
    if prompt is None:
        raise ValueError("prompt is required")
    image_ref = image_path or nonempty(fields.get("image_path")) or nonempty(fields.get("input_reference"))
    task = nonempty(fields.get("task"))
    if task not in SUPPORTED_TASKS:
        task = "ref2va" if image_ref else "t2va"
    payload = {
        "prompt": prompt,
        "task": task,
        "target": build_target(fields),
    }
    if image_ref and task == "ref2va":
        payload["input_reference"] = image_ref
    seed = whole_number(fields.get("seed"))
    if seed is not None:
        payload["seed"] = seed
    steps = whole_number(fields.get("num_inference_steps"))
    if steps is None:
        steps = whole_number(fields.get("infer_steps"))
    if steps is not None and steps > 0:
        payload["num_inference_steps"] = steps
    width, height = dimensions(fields)
    if width and height:
        payload["width"] = width
        payload["height"] = height
        payload["size"] = "%dx%d" % (width, height)
    payload["seconds"] = duration_seconds(fields)
    return payload


def map_status(status):
    key = str(status or "").strip().lower()
    if key in ("queued", "submitted", "pending"):
        return "submitted"
    if key in ("in_progress", "processing", "running"):
        return "running"
    if key in ("completed", "succeed", "succeeded", "success"):
        return "succeed"
    if key in ("failed", "error", "cancelled", "canceled"):
        return "failed"
    return key or "submitted"


def map_progress(value):
    number = finite_number(value)
    if number is None:
        return None
    if number > 1:
        number = number / 100.0
    return max(0.0, min(1.0, number))


def error_message(payload):
    err = payload.get("error")
    if isinstance(err, dict):
        return nonempty(err.get("message")) or nonempty(err.get("detail")) or ""
    if isinstance(err, str):
        return err
    return ""


def lightx2v_create_response(payload):
    return {
        "task_id": payload.get("id"),
        "status": map_status(payload.get("status")),
        "message": "Task created successfully",
    }


def lightx2v_status_response(payload):
    task_id = payload.get("id")
    status = map_status(payload.get("status"))
    body = {
        "task_id": task_id,
        "status": status,
        "message": error_message(payload) if status == "failed" else "Processing...",
    }
    if status == "succeed":
        body["message"] = ""
        if task_id:
            body["file_path"] = "outputs/videos/%s.mp4" % task_id
    progress = map_progress(payload.get("progress"))
    if progress is not None:
        body["progress"] = progress
    return body


def video_id_from_download(path):
    name = path.rstrip("/").split("/")[-1]
    if name.endswith(".mp4"):
        name = name[:-4]
    if not name or name in (".", ".."):
        return None
    return name


def disposition_param(disposition, key):
    prefix = key + "="
    for item in disposition.split(";"):
        item = item.strip()
        if item.startswith(prefix):
            return item[len(prefix):].strip().strip('"')
    return None


def parse_multipart(body, boundary):
    fields = {}
    files = {}
    marker = b"--" + boundary.encode("utf-8")
    for part in body.split(marker):
        part = part.strip(b"\r\n")
        if not part or part == b"--":
            continue
        if part.endswith(b"--"):
            part = part[:-2].rstrip(b"\r\n")
        if b"\r\n\r\n" not in part:
            continue
        raw_headers, content = part.split(b"\r\n\r\n", 1)
        disposition = ""
        for line in raw_headers.decode("utf-8", "replace").split("\r\n"):
            if line.lower().startswith("content-disposition:"):
                disposition = line.split(":", 1)[1]
        name = disposition_param(disposition, "name")
        filename = disposition_param(disposition, "filename")
        if not name:
            continue
        if filename:
            files[name] = (filename, content)
        else:
            fields[name] = content.decode("utf-8", "replace")
    return fields, files


def prune_old_uploads(directory):
    now = time.time()
    try:
        names = os.listdir(directory)
    except OSError:
        return
    for name in names:
        path = os.path.join(directory, name)
        try:
            if now - os.path.getmtime(path) > UPLOAD_MAX_AGE_SECONDS:
                os.remove(path)
        except OSError:
            continue


def save_upload(filename, content):
    os.makedirs(UPLOAD_DIR, exist_ok=True)
    prune_old_uploads(UPLOAD_DIR)
    suffix = os.path.splitext(filename or "")[1]
    handle = tempfile.NamedTemporaryFile(delete=False, dir=UPLOAD_DIR, suffix=suffix)
    try:
        handle.write(content)
    finally:
        handle.close()
    return handle.name


def read_create_fields(content_type, body):
    mime = content_type.split(";", 1)[0].strip().lower()
    if mime.startswith("multipart/"):
        boundary = ""
        for item in content_type.split(";")[1:]:
            item = item.strip()
            if item.lower().startswith("boundary="):
                boundary = item.split("=", 1)[1].strip().strip('"')
        if not boundary:
            raise ValueError("multipart body is missing a boundary")
        fields, files = parse_multipart(body, boundary)
        image = files.get("image_file") or files.get("input_reference")
        image_path = save_upload(image[0], image[1]) if image else None
        return fields, image_path
    if mime == "application/x-www-form-urlencoded":
        return dict(parse_qsl(body.decode("utf-8"), keep_blank_values=True)), None
    if not body:
        return {}, None
    payload = json.loads(body.decode("utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("JSON body must be an object")
    return payload, None


def header_value(scope, name):
    for key, value in scope.get("headers") or []:
        if key.decode("latin-1").lower() == name:
            return value.decode("latin-1")
    return ""


def with_path(scope, path, method=None, headers=None):
    inner = dict(scope)
    inner["path"] = path
    inner["raw_path"] = path.encode("utf-8")
    if method is not None:
        inner["method"] = method
        inner["query_string"] = b""
    if headers is not None:
        inner["headers"] = headers
    return inner


async def read_body(receive):
    chunks = []
    while True:
        message = await receive()
        if message["type"] != "http.request":
            break
        chunks.append(message.get("body", b""))
        if not message.get("more_body"):
            break
    return b"".join(chunks)


async def send_json(send, status, payload):
    body = json.dumps(payload).encode("utf-8")
    await send({
        "type": "http.response.start",
        "status": status,
        "headers": [
            (b"content-type", b"application/json"),
            (b"content-length", str(len(body)).encode("latin-1")),
        ],
    })
    await send({"type": "http.response.body", "body": body})


class LightX2VCompatMiddleware:
    """Serve LightX2V video routes by calling the wrapped SGLang app."""

    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        method = scope["method"]
        path = scope["path"].rstrip("/") or "/"
        if method == "POST" and path in CREATE_PATHS:
            await self.create(scope, receive, send)
            return
        if method == "GET" and path.startswith("/v1/tasks/"):
            parts = path.split("/")
            if len(parts) == 5 and parts[4] == "status":
                await self.status(scope, receive, send, parts[3])
                return
            if len(parts) == 5 and parts[4] == "result":
                await self.app(with_path(scope, "/v1/videos/%s/content" % parts[3]), receive, send)
                return
        if method == "GET" and path.startswith(DOWNLOAD_PREFIX):
            video_id = video_id_from_download(path)
            if video_id:
                await self.app(with_path(scope, "/v1/videos/%s/content" % video_id), receive, send)
                return
        await self.app(scope, receive, send)

    async def create(self, scope, receive, send):
        try:
            body = await read_body(receive)
            fields, image_path = read_create_fields(header_value(scope, "content-type"), body)
            payload = lightx2v_to_sglang(fields, image_path)
        except (ValueError, json.JSONDecodeError) as exc:
            await send_json(send, 400, {"detail": str(exc)})
            return
        status, raw = await self.call(scope, receive, "POST", "/v1/videos", json.dumps(payload).encode("utf-8"))
        await self.reply(send, status, raw, lightx2v_create_response)

    async def status(self, scope, receive, send, task_id):
        status, raw = await self.call(scope, receive, "GET", "/v1/videos/" + task_id, b"")
        await self.reply(send, status, raw, lightx2v_status_response)

    async def reply(self, send, status, raw, convert):
        if status == 200:
            try:
                await send_json(send, 200, convert(json.loads(raw.decode("utf-8"))))
                return
            except (ValueError, AttributeError):
                pass
        await send({
            "type": "http.response.start",
            "status": status,
            "headers": [(b"content-type", b"application/json"), (b"content-length", str(len(raw)).encode("latin-1"))],
        })
        await send({"type": "http.response.body", "body": raw})

    async def call(self, scope, receive, method, path, body):
        headers = [
            (key, value)
            for key, value in scope.get("headers") or []
            if key.lower() not in (b"content-type", b"content-length")
        ]
        if body:
            headers += [(b"content-type", b"application/json"), (b"content-length", str(len(body)).encode("latin-1"))]
        delivered = False

        async def inner_receive():
            nonlocal delivered
            if not delivered:
                delivered = True
                return {"type": "http.request", "body": body, "more_body": False}
            return await receive()

        result = {"status": 500, "body": []}

        async def inner_send(message):
            if message["type"] == "http.response.start":
                result["status"] = message["status"]
            elif message["type"] == "http.response.body":
                result["body"].append(message.get("body", b""))

        await self.app(with_path(scope, path, method, headers), inner_receive, inner_send)
        return result["status"], b"".join(result["body"])


def install():
    from sglang.multimodal_gen.runtime import launch_server

    create_app = launch_server.create_app

    def create_app_with_compat(server_args):
        app = create_app(server_args)
        app.add_middleware(LightX2VCompatMiddleware)
        return app

    launch_server.create_app = create_app_with_compat


def main(argv):
    install()
    from sglang.cli.main import main as sglang_main

    sys.argv = ["sglang", "serve", *argv]
    sglang_main()


if __name__ == "__main__":
    main(sys.argv[1:])
