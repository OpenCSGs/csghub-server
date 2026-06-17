import argparse
import base64
import inspect
import io
import logging
import os
import time
from typing import Any, Dict, List, Optional

import torch
import uvicorn
from diffusers import DiffusionPipeline
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse
from PIL import Image


logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

app = FastAPI(title="OpenCSG Diffusers API", version="0.38.0")

PIPELINE = None
MODEL_ID = "local"
MODEL_PATH = ""
DEVICE = "cuda"


def resolve_dtype(device: str) -> torch.dtype:
    if device == "cuda":
        if torch.cuda.is_available() and torch.cuda.is_bf16_supported():
            return torch.bfloat16
        return torch.float16
    return torch.float32


def normalize_device(device: str) -> str:
    requested = (device or "").lower()
    if requested == "cuda" and torch.cuda.is_available():
        return "cuda"
    if requested == "mps" and getattr(torch.backends, "mps", None) is not None and torch.backends.mps.is_available():
        return "mps"
    return "cpu"


def parse_size(value: Optional[str]) -> tuple[Optional[int], Optional[int]]:
    if not value or value == "auto":
        return None, None
    parts = value.lower().split("x", 1)
    if len(parts) != 2:
        raise HTTPException(status_code=400, detail="size must be WIDTHxHEIGHT")
    try:
        width = int(parts[0])
        height = int(parts[1])
    except ValueError as exc:
        raise HTTPException(status_code=400, detail="size must be WIDTHxHEIGHT") from exc
    if width <= 0 or height <= 0:
        raise HTTPException(status_code=400, detail="size must be positive")
    return width, height


async def read_image(file: UploadFile) -> Image.Image:
    try:
        content = await file.read()
        image = Image.open(io.BytesIO(content))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"{file.filename or 'image'} is not a valid image") from exc
    return image.convert("RGB")


def load_pipeline(model_path: str, device: str):
    if not os.path.exists(model_path):
        raise FileNotFoundError(f"local model path does not exist: {model_path}")
    dtype = resolve_dtype(device)
    logger.info("Loading diffusers pipeline from %s on %s with dtype=%s", model_path, device, dtype)
    started = time.time()
    pipe = DiffusionPipeline.from_pretrained(
        model_path,
        torch_dtype=dtype,
        local_files_only=True,
        trust_remote_code=True,
    )
    if device == "cuda":
        pipe = pipe.to("cuda")
        if hasattr(pipe, "enable_model_cpu_offload"):
            pipe.enable_model_cpu_offload()
    elif device == "mps":
        pipe = pipe.to("mps")
    else:
        pipe = pipe.to("cpu")
    if hasattr(pipe, "enable_attention_slicing"):
        pipe.enable_attention_slicing()
    if hasattr(pipe, "enable_vae_tiling"):
        pipe.enable_vae_tiling()
    logger.info("Loaded diffusers pipeline in %.1fs", time.time() - started)
    return pipe


def build_kwargs(
    prompt: str,
    images: List[Image.Image],
    mask_image: Optional[Image.Image],
    size: Optional[str],
    n: Optional[int],
    negative_prompt: Optional[str],
    steps: Optional[int],
    cfg_scale: Optional[float],
    seed: Optional[int],
) -> Dict[str, Any]:
    if PIPELINE is None:
        raise HTTPException(status_code=503, detail="model is not loaded")
    signature = inspect.signature(PIPELINE.__call__)
    params = signature.parameters
    kwargs: Dict[str, Any] = {"prompt": prompt}

    if "image" in params:
        if not images and params["image"].default is inspect.Parameter.empty:
            raise HTTPException(status_code=400, detail="image is required for this model")
        if images:
            kwargs["image"] = images if len(images) > 1 else images[0]

    if mask_image is not None:
        if "mask_image" in params:
            kwargs["mask_image"] = mask_image
        elif "mask" in params:
            kwargs["mask"] = mask_image

    width, height = parse_size(size)
    if width and height and "width" in params and "height" in params:
        kwargs["width"] = width
        kwargs["height"] = height
    if "num_images_per_prompt" in params:
        kwargs["num_images_per_prompt"] = max(1, min(n or 1, 4))
    if negative_prompt and "negative_prompt" in params:
        kwargs["negative_prompt"] = negative_prompt
    if steps and "num_inference_steps" in params:
        kwargs["num_inference_steps"] = steps
    if cfg_scale is not None:
        if "true_cfg_scale" in params:
            kwargs["true_cfg_scale"] = cfg_scale
        elif "guidance_scale" in params:
            kwargs["guidance_scale"] = cfg_scale
    if "true_cfg_scale" in kwargs and "guidance_scale" in params and "guidance_scale" not in kwargs:
        kwargs["guidance_scale"] = 1.0
    if seed is not None and "generator" in params:
        generator_device = DEVICE if DEVICE != "mps" else "cpu"
        kwargs["generator"] = torch.Generator(device=generator_device).manual_seed(seed)
    return kwargs


def encode_response(images: List[Image.Image], size: Optional[str], output_format: Optional[str]) -> JSONResponse:
    image_format = (output_format or "png").lower()
    if image_format == "jpg":
        image_format = "jpeg"
    if image_format not in {"png", "jpeg", "webp"}:
        image_format = "png"

    data = []
    for image in images:
        buf = io.BytesIO()
        image.save(buf, format=image_format.upper())
        data.append({"b64_json": base64.b64encode(buf.getvalue()).decode("ascii")})

    payload: Dict[str, Any] = {"created": int(time.time()), "data": data}
    if size:
        payload["size"] = size
    payload["output_format"] = image_format
    return JSONResponse(payload)


async def run_image_request(
    prompt: str,
    image: Optional[List[UploadFile]],
    mask: Optional[UploadFile],
    size: Optional[str],
    n: Optional[int],
    negative_prompt: Optional[str],
    steps: Optional[int],
    cfg_scale: Optional[float],
    seed: Optional[int],
    output_format: Optional[str],
) -> JSONResponse:
    if not prompt:
        raise HTTPException(status_code=400, detail="prompt is required")
    input_images = [await read_image(file) for file in (image or [])]
    input_mask = await read_image(mask) if mask is not None else None
    kwargs = build_kwargs(prompt, input_images, input_mask, size, n, negative_prompt, steps, cfg_scale, seed)
    started = time.time()
    with torch.inference_mode():
        result = PIPELINE(**kwargs)
    logger.info("Generated %s image(s) in %.1fs", len(result.images), time.time() - started)
    return encode_response(result.images, size, output_format)


@app.post("/")
@app.post("/v1/images/edits")
async def edit_image(
    image: List[UploadFile] = File(...),
    mask: Optional[UploadFile] = File(default=None),
    model: str = Form(default="local"),
    prompt: str = Form(...),
    size: Optional[str] = Form(default=None),
    n: Optional[int] = Form(default=1),
    negative_prompt: Optional[str] = Form(default=None),
    steps: Optional[int] = Form(default=None),
    cfg_scale: Optional[float] = Form(default=None),
    seed: Optional[int] = Form(default=None),
    output_format: Optional[str] = Form(default=None),
    response_format: Optional[str] = Form(default="b64_json"),
):
    _ = model
    _ = response_format
    return await run_image_request(prompt, image, mask, size, n, negative_prompt, steps, cfg_scale, seed, output_format)


@app.post("/v1/images/generations")
async def generate_image(
    model: str = Form(default="local"),
    prompt: str = Form(...),
    size: Optional[str] = Form(default=None),
    n: Optional[int] = Form(default=1),
    negative_prompt: Optional[str] = Form(default=None),
    steps: Optional[int] = Form(default=None),
    cfg_scale: Optional[float] = Form(default=None),
    seed: Optional[int] = Form(default=None),
    output_format: Optional[str] = Form(default=None),
    response_format: Optional[str] = Form(default="b64_json"),
):
    _ = model
    _ = response_format
    return await run_image_request(prompt, [], None, size, n, negative_prompt, steps, cfg_scale, seed, output_format)


@app.get("/v1/models")
async def list_models():
    aliases = ["local", MODEL_ID]
    repo_name = MODEL_ID.rsplit("/", 1)[-1]
    if repo_name not in aliases:
        aliases.append(repo_name)
    return {
        "object": "list",
        "data": [
            {
                "id": alias,
                "object": "model",
                "created": 1700000000,
                "owned_by": "opencsg",
                "ready": PIPELINE is not None,
            }
            for alias in aliases
        ],
    }


@app.get("/health")
async def health():
    return {
        "status": "ok" if PIPELINE is not None else "loading",
        "device": DEVICE,
        "model_id": MODEL_ID,
        "model_path": MODEL_PATH,
    }


def main():
    parser = argparse.ArgumentParser(description="Diffusers image edit API server")
    parser.add_argument("--host", default="0.0.0.0", help="Bind host")
    parser.add_argument("--port", type=int, default=8000, help="Bind port")
    parser.add_argument("--device", default=os.getenv("DIFFUSERS_DEVICE", "cuda"), help="Device: cuda, cpu, mps")
    parser.add_argument("--model-path", required=True, help="Local model directory")
    parser.add_argument("--model-id", default="local", help="Model id exposed by the API")
    args, unknown_args = parser.parse_known_args()
    if unknown_args:
        logger.warning("Ignoring unsupported server args: %s", unknown_args)

    global PIPELINE, DEVICE, MODEL_ID, MODEL_PATH
    DEVICE = normalize_device(args.device)
    MODEL_ID = args.model_id
    MODEL_PATH = args.model_path
    PIPELINE = load_pipeline(args.model_path, DEVICE)

    logger.info("Diffusers API server starting on http://%s:%s", args.host, args.port)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
