"""OpenAI-compatible speech API server for iFLYTEK AudioFly.

AudioFly is a latent-diffusion text-to-audio model that ships its own
inference code (the `ldm` package) inside the model repository. This server
wraps it with the same speech API surface as vLLM-Omni so the CSGHub
aigateway can proxy /v1/audio/speech requests to it unchanged:

  POST /v1/audio/speech        -> binary wav audio
  POST /v1/audio/speech/batch  -> JSON with base64-encoded results
  GET  /v1/audio/voices        -> empty voice list (model has no voices)
  GET  /health                 -> liveness probe

The model repo root (containing ldm/, config/, models/) must be the working
directory and on PYTHONPATH before this module is imported; serve.sh takes
care of both.
"""

import base64
import glob
import logging
import os
import threading
import uuid

import torch
import yaml
from fastapi import FastAPI
from fastapi.responses import JSONResponse, Response

from generation import (
    DEFAULT_CFG,
    DEFAULT_DDIM_STEPS,
    parse_generation_params,
    wav_duration_seconds,
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("audiofly")

MODEL_ID = os.environ.get("REPO_ID", "AudioFly")
CONFIG_PATH = os.environ.get("AUDIOFLY_CONFIG", "./config/config.yaml")
CHECKPOINT_PATH = os.environ.get("AUDIOFLY_CHECKPOINT", "./models/ldm/model.ckpt")
OUTPUT_DIR = os.environ.get("AUDIOFLY_OUTPUT_DIR", "/tmp/audiofly")

app = FastAPI()

model = None
# The diffusion sampling loop is GPU-heavy; serialize generations to avoid
# concurrent requests exhausting GPU memory.
generate_lock = threading.Lock()


def load_model():
    global model
    from ldm.utils.util import instantiate_from_config

    logger.info("loading AudioFly model from %s", CHECKPOINT_PATH)
    with open(CONFIG_PATH, "r") as f:
        configs = yaml.load(f, Loader=yaml.FullLoader)
    model = instantiate_from_config(configs["model"])
    checkpoint = torch.load(CHECKPOINT_PATH, map_location="cpu")
    model.load_state_dict(checkpoint, strict=False)
    model.eval()
    if torch.cuda.is_available():
        model = model.cuda()
    logger.info("AudioFly model loaded")


@app.on_event("startup")
def startup():
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    load_model()


def error_response(status_code: int, message: str, code: str = "invalid_request_error"):
    return JSONResponse(
        status_code=status_code,
        content={"error": {"code": code, "message": message, "type": code}},
    )


def generate_audio(text: str, cfg: float, ddim_steps: int) -> bytes:
    """Runs one diffusion generation and returns the wav file bytes."""
    name = uuid.uuid4().hex
    outputdir = os.path.join(OUTPUT_DIR, name)
    os.makedirs(outputdir, exist_ok=True)
    try:
        with generate_lock, torch.no_grad():
            model.generate_sample(
                textlist=[text],
                name=name,
                cfg=cfg,
                ddim_steps=ddim_steps,
                outputdir=outputdir,
            )
        wavs = sorted(glob.glob(os.path.join(outputdir, "**", "*.wav"), recursive=True))
        if not wavs:
            raise RuntimeError("generation produced no audio file")
        with open(wavs[0], "rb") as f:
            return f.read()
    finally:
        for path in glob.glob(os.path.join(outputdir, "**", "*"), recursive=True):
            if os.path.isfile(path):
                os.remove(path)
        if os.path.isdir(outputdir):
            os.removedirs(outputdir)


@app.post("/v1/audio/speech")
def speech(payload: dict):
    params, err = parse_generation_params(payload)
    if err:
        return error_response(400, err)
    try:
        audio = generate_audio(params["text"], params["cfg"], params["ddim_steps"])
    except Exception:
        logger.exception("audio generation failed")
        return error_response(500, "audio generation failed", code="internal_error")
    duration = wav_duration_seconds(audio)
    return Response(
        content=audio,
        media_type="audio/wav",
        headers={"Audio-Duration-Seconds": str(duration)},
    )


@app.post("/v1/audio/speech/batch")
def speech_batch(payload: dict):
    items = payload.get("items")
    if not isinstance(items, list) or not items:
        return error_response(400, "items cannot be empty")

    results = []
    for index, item in enumerate(items):
        if not isinstance(item, dict):
            results.append({"index": index, "status": "error", "error": "item must be an object"})
            continue
        # Batch-level defaults apply to every item unless overridden.
        merged = {k: v for k, v in payload.items() if k != "items"}
        merged.update(item)
        params, err = parse_generation_params(merged)
        if err:
            results.append({"index": index, "status": "error", "error": err})
            continue
        try:
            audio = generate_audio(params["text"], params["cfg"], params["ddim_steps"])
        except Exception:
            logger.exception("audio generation failed for batch item %d", index)
            results.append({"index": index, "status": "error", "error": "audio generation failed"})
            continue
        duration = wav_duration_seconds(audio)
        # AudioFly has no token accounting; report input characters so the
        # gateway can meter usage consistently.
        chars = len(params["text"])
        results.append({
            "index": index,
            "status": "success",
            "audio": base64.b64encode(audio).decode("ascii"),
            "response_format": "wav",
            "usage": {
                "input_tokens": chars,
                "output_tokens": 0,
                "total_tokens": chars,
                "seconds": duration,
            },
        })
    return {"object": "list", "model": MODEL_ID, "results": results}


@app.get("/v1/audio/voices")
def list_voices():
    # AudioFly is a text-to-audio (sound effect) model without voice presets
    # or voice cloning; expose an empty list for API compatibility.
    return {"voices": []}


@app.post("/v1/audio/voices")
@app.put("/v1/audio/voices")
@app.delete("/v1/audio/voices/{name}")
def voices_not_supported(name: str = ""):
    return error_response(405, "voice management is not supported by this model", code="not_supported")


@app.get("/health")
def health():
    return {"status": "ok" if model is not None else "loading"}
