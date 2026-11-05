import asyncio
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
import video_compat


class FakeSGLang:
    """Records calls and answers like the SGLang /v1/videos routes."""

    def __init__(self):
        self.requests = []

    async def __call__(self, scope, receive, send):
        body = b""
        if scope["method"] == "POST":
            message = await receive()
            body = message.get("body", b"")
        self.requests.append((scope["method"], scope["path"], dict(scope["headers"]), body))
        path = scope["path"]
        if path == "/v1/videos":
            status, content_type, payload = 200, b"application/json", json.dumps({"id": "vid-1", "status": "queued", "progress": 0}).encode()
        elif path.endswith("/content"):
            status, content_type, payload = 200, b"video/mp4", b"mp4-bytes"
        elif path.startswith("/v1/videos/missing"):
            status, content_type, payload = 404, b"application/json", b'{"detail":"Video not found"}'
        elif path.startswith("/v1/videos/"):
            status, content_type, payload = 200, b"application/json", json.dumps({"id": "vid-1", "status": "completed", "progress": 100}).encode()
        else:
            status, content_type, payload = 200, b"application/json", b'{"object":"list"}'
        await send({"type": "http.response.start", "status": status, "headers": [(b"content-type", content_type)]})
        await send({"type": "http.response.body", "body": payload})


def request(app, method, path, body=b"", content_type=None):
    headers = [(b"authorization", b"Bearer token")]
    if content_type:
        headers.append((b"content-type", content_type.encode()))
    scope = {"type": "http", "method": method, "path": path, "raw_path": path.encode(), "query_string": b"", "headers": headers}
    sent = {"body": b""}

    async def receive():
        return {"type": "http.request", "body": body, "more_body": False}

    async def send(message):
        if message["type"] == "http.response.start":
            sent["status"] = message["status"]
            sent["headers"] = dict(message["headers"])
        else:
            sent["body"] += message.get("body", b"")

    asyncio.run(app(scope, receive, send))
    return sent


class TranslateTest(unittest.TestCase):
    def test_lightx2v_json_maps_onto_sglang_target(self):
        payload = video_compat.lightx2v_to_sglang({
            "prompt": "A white cat sitting by the beach",
            "negative_prompt": "blurry, low quality",
            "infer_steps": 4,
            "video_duration": 5,
            "seed": 42,
            "height": 480,
            "width": 832,
        })
        self.assertEqual(payload, {
            "prompt": "A white cat sitting by the beach",
            "task": "t2va",
            "target": {"short_edge": 480, "aspect_ratio": "16:9", "duration_seconds": 5},
            "seed": 42,
            "num_inference_steps": 4,
            "width": 832,
            "height": 480,
            "size": "832x480",
            "seconds": 5,
        })

    def test_defaults_and_caller_task_and_target(self):
        defaults = video_compat.lightx2v_to_sglang({"prompt": "a fox"})
        self.assertEqual(defaults["target"], {"short_edge": 768, "aspect_ratio": "16:9", "duration_seconds": 5})
        kept = video_compat.lightx2v_to_sglang({
            "prompt": "a fox",
            "task": "fl2va",
            "target": {"short_edge": 512, "aspect_ratio": "1:1", "duration_seconds": 3},
        })
        self.assertEqual(kept["task"], "fl2va")
        self.assertEqual(kept["target"]["short_edge"], 512)

    def test_portrait_aspect(self):
        target = video_compat.lightx2v_to_sglang({"prompt": "a fox", "size": "720x1280", "video_duration": 10})["target"]
        self.assertEqual(target, {"short_edge": 720, "aspect_ratio": "9:16", "duration_seconds": 10})

    def test_image_selects_ref2va(self):
        payload = video_compat.lightx2v_to_sglang({"prompt": "a cat"}, image_path="/tmp/cat.jpg")
        self.assertEqual(payload["task"], "ref2va")
        self.assertEqual(payload["input_reference"], "/tmp/cat.jpg")

    def test_prompt_required(self):
        with self.assertRaises(ValueError):
            video_compat.lightx2v_to_sglang({})

    def test_status_mapping(self):
        done = video_compat.lightx2v_status_response({"id": "vid-1", "status": "completed", "progress": 100})
        self.assertEqual(done, {"task_id": "vid-1", "status": "succeed", "message": "", "file_path": "outputs/videos/vid-1.mp4", "progress": 1.0})
        queued = video_compat.lightx2v_status_response({"id": "vid-1", "status": "queued", "progress": 40})
        self.assertEqual((queued["status"], queued["progress"]), ("submitted", 0.4))
        failed = video_compat.lightx2v_status_response({"id": "vid-1", "status": "failed", "error": {"message": "boom"}})
        self.assertEqual((failed["status"], failed["message"]), ("failed", "boom"))


class MiddlewareTest(unittest.TestCase):
    def setUp(self):
        self.sglang = FakeSGLang()
        self.app = video_compat.LightX2VCompatMiddleware(self.sglang)

    def test_create_translates_onto_videos(self):
        body = json.dumps({"prompt": "a cat walking", "video_duration": 5, "negative_prompt": "blurry"}).encode()
        resp = request(self.app, "POST", "/v1/tasks/video/", body, "application/json")
        self.assertEqual(resp["status"], 200)
        self.assertEqual(json.loads(resp["body"]), {"task_id": "vid-1", "status": "submitted", "message": "Task created successfully"})
        method, path, headers, forwarded = self.sglang.requests[-1]
        self.assertEqual((method, path), ("POST", "/v1/videos"))
        self.assertEqual(headers[b"authorization"], b"Bearer token")
        self.assertEqual(headers[b"content-type"], b"application/json")
        forwarded = json.loads(forwarded)
        self.assertEqual(forwarded["task"], "t2va")
        self.assertNotIn("negative_prompt", forwarded)

    def test_create_rejects_missing_prompt(self):
        resp = request(self.app, "POST", "/v1/tasks/video", b"{}", "application/json")
        self.assertEqual(resp["status"], 400)
        self.assertEqual(self.sglang.requests, [])

    def test_status_and_download(self):
        status = request(self.app, "GET", "/v1/tasks/vid-1/status")
        self.assertEqual(json.loads(status["body"])["status"], "succeed")
        self.assertEqual(self.sglang.requests[-1][1], "/v1/videos/vid-1")

        download = request(self.app, "GET", "/v1/files/download/outputs/videos/vid-1.mp4")
        self.assertEqual(download["headers"][b"content-type"], b"video/mp4")
        self.assertEqual(download["body"], b"mp4-bytes")
        self.assertEqual(self.sglang.requests[-1][1], "/v1/videos/vid-1/content")

    def test_status_passes_upstream_error_through(self):
        resp = request(self.app, "GET", "/v1/tasks/missing/status")
        self.assertEqual(resp["status"], 404)
        self.assertEqual(json.loads(resp["body"]), {"detail": "Video not found"})

    def test_other_routes_pass_through(self):
        resp = request(self.app, "GET", "/v1/models")
        self.assertEqual(json.loads(resp["body"]), {"object": "list"})
        self.assertEqual(self.sglang.requests[-1][1], "/v1/models")

    def test_urlencoded_form_create(self):
        resp = request(self.app, "POST", "/v1/tasks/video/form", b"prompt=a+dog&video_duration=3", "application/x-www-form-urlencoded")
        self.assertEqual(resp["status"], 200)
        forwarded = json.loads(self.sglang.requests[-1][3])
        self.assertEqual(forwarded["prompt"], "a dog")
        self.assertEqual(forwarded["target"]["duration_seconds"], 3)
        self.assertIsInstance(forwarded["target"]["duration_seconds"], int)

    def test_form_upload_becomes_ref2va(self):
        boundary = "compat"
        body = (
            "--{b}\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\na cat walking\r\n"
            "--{b}\r\nContent-Disposition: form-data; name=\"image_file\"; filename=\"cat.jpg\"\r\n"
            "Content-Type: image/jpeg\r\n\r\njpeg-bytes\r\n--{b}--\r\n"
        ).format(b=boundary).encode()
        resp = request(self.app, "POST", "/v1/tasks/video/form", body, "multipart/form-data; boundary=" + boundary)
        self.assertEqual(resp["status"], 200)
        forwarded = json.loads(self.sglang.requests[-1][3])
        self.assertEqual(forwarded["task"], "ref2va")
        with open(forwarded["input_reference"], "rb") as handle:
            self.assertEqual(handle.read(), b"jpeg-bytes")
        os.remove(forwarded["input_reference"])


if __name__ == "__main__":
    unittest.main()
