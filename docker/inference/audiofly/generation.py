import io
import wave


DEFAULT_CFG = 3.5
DEFAULT_DDIM_STEPS = 200


def wav_duration_seconds(audio: bytes) -> float:
    """Return the duration of a WAV payload in seconds."""
    with wave.open(io.BytesIO(audio), "rb") as wav:
        frame_rate = wav.getframerate()
        if frame_rate <= 0:
            return 0
        return wav.getnframes() / frame_rate


def parse_generation_params(payload: dict):
    """Validate one generation request and extract AudioFly parameters."""
    text = payload.get("input")
    if not isinstance(text, str) or not text.strip():
        return None, "input cannot be empty"
    if payload.get("stream"):
        return None, "streaming is not supported by this model"
    response_format = payload.get("response_format") or "wav"
    if response_format != "wav":
        return None, f"unsupported response_format {response_format!r}, only 'wav' is supported"
    try:
        cfg = float(payload.get("cfg", DEFAULT_CFG))
        ddim_steps = int(payload.get("ddim_steps", DEFAULT_DDIM_STEPS))
    except (TypeError, ValueError):
        return None, "cfg must be a number and ddim_steps must be an integer"
    if ddim_steps < 1 or ddim_steps > 1000:
        return None, "ddim_steps must be between 1 and 1000"
    return {"text": text, "cfg": cfg, "ddim_steps": ddim_steps}, None
