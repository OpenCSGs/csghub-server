"""Patch vLLM-Omni v0.31.0rc1 to expose generated TTS duration.

The patch is intentionally version-locked. It must fail during the image
build when vLLM-Omni changes, rather than silently producing an image without
the duration field.
"""

import importlib.util
from pathlib import Path


def replace_once(path: Path, old: str, new: str) -> None:
    content = path.read_text()
    if content.count(old) != 1:
        raise RuntimeError(f"unexpected vLLM-Omni v0.31.0rc1 source in {path}")
    path.write_text(content.replace(old, new))


# Do not import vllm_omni: v0.31.0rc1 loads CUDA libcudart at import time,
# which is missing in the ROCm image.
spec = importlib.util.find_spec("vllm_omni")
if spec is None or spec.origin is None:
    raise RuntimeError("vllm_omni is not installed")
serving_path = Path(spec.origin).resolve().parent / "entrypoints/openai/serving_speech.py"

replace_once(
    serving_path,
    """        first_audio_chunk_s: float | None = None
        first_audio_packet_ts: float | None = None
        ttfp_observed = False
        stream_start_s = request_start_s if request_start_s is not None else time.perf_counter()
        artifact_ready = False
""",
    """        first_audio_chunk_s: float | None = None
        first_audio_packet_ts: float | None = None
        ttfp_observed = False
        total_audio_samples = 0
        stream_start_s = request_start_s if request_start_s is not None else time.perf_counter()
        artifact_ready = False
""",
)

# v0.31.0rc1 slices the cumulative tensor and resamples *after* the squeeze
# above, so the sample tally is taken once the chunk that is actually encoded
# is final. Resampling preserves duration, so counting source-rate samples and
# dividing by ``sample_rate_val`` stays exact (the resampler's held-back tail
# is emitted by the final flush from samples already counted here).
replace_once(
    serving_path,
    """                    if resampler is not None:
                        chunk_np = resampler.process(chunk_np)
                        if chunk_np.size == 0:
                            continue
""",
    """                    chunk_array = np.asarray(chunk_np)
                    total_audio_samples += int(chunk_array.size // max(_infer_audio_num_channels(chunk_array), 1))
                    if resampler is not None:
                        chunk_np = resampler.process(chunk_np)
                        if chunk_np.size == 0:
                            continue
""",
)

replace_once(
    serving_path,
    """            else:
                logger.info(
                    "[SpeechE2E] request_id=%s stream=true status=ok total_ms=%.2f first_chunk_ms=NA",
                    request_id,
                    total_ms,
                )
        except GeneratorExit:
            record_stream_abort("closed")
            raise
""",
    """            else:
                logger.info(
                    "[SpeechE2E] request_id=%s stream=true status=ok total_ms=%.2f first_chunk_ms=NA",
                    request_id,
                    total_ms,
                )
            if raw_request is not None and total_audio_samples > 0:
                raw_request.state.audio_duration_s = total_audio_samples / sample_rate_val
        except GeneratorExit:
            record_stream_abort("closed")
            raise
""",
)

replace_once(
    serving_path,
    """        The terminal ``speech.audio.done`` event carries a ``usage`` object
        (``input_tokens``/``output_tokens``/``total_tokens`` + a per-modality
        ``input_token_details`` breakdown), matching OpenAI's documented
        ``speech.audio.done`` schema. ``output_tokens`` is accumulated from the
        stage-0 deltas as they stream (see ``SpeechOutputTokenCounter``);
        ``input_tokens`` is computed from the request text + reference audio.
""",
    """        The terminal ``speech.audio.done`` event carries a ``usage`` object
        (``input_tokens``/``output_tokens``/``total_tokens`` + a per-modality
        ``input_token_details`` breakdown), matching OpenAI's documented
        ``speech.audio.done`` schema. ``output_tokens`` is accumulated from the
        stage-0 deltas as they stream (see ``SpeechOutputTokenCounter``);
        ``input_tokens`` is computed from the request text + reference audio.
        ``duration`` is the generated audio duration in seconds.
""",
)

replace_once(
    serving_path,
    """            done_payload: dict[str, Any] = {"type": "speech.audio.done"}
            if request is not None:
                # Streaming path: output_tokens = sum of stage-0 deltas.
                usage = self._build_speech_usage(request, tts_params or {}, usage_acc.total())
                done_payload["usage"] = usage.model_dump()
""",
    """            done_payload: dict[str, Any] = {"type": "speech.audio.done"}
            duration_s = getattr(raw_request.state, "audio_duration_s", None) if raw_request is not None else None
            if duration_s is not None:
                done_payload["duration"] = round(duration_s, 3)
            if request is not None:
                # Streaming path: output_tokens = sum of stage-0 deltas.
                usage = self._build_speech_usage(request, tts_params or {}, usage_acc.total())
                done_payload["usage"] = usage.model_dump()
""",
)

replace_once(
    serving_path,
    """        usage_out: list[SpeechTokenUsage] | None = None,
        has_inline_ref_audio: bool | None = None,
        collect: dict | None = None,
        request_arrival_ts: float | None = None,
    ) -> tuple[bytes | str, str]:
""",
    """        raw_request: Request | None = None,
        usage_out: list[SpeechTokenUsage] | None = None,
        has_inline_ref_audio: bool | None = None,
        collect: dict | None = None,
        request_arrival_ts: float | None = None,
    ) -> tuple[bytes | str, str]:
""",
)

replace_once(
    serving_path,
    """            audio_obj = CreateAudio(
                audio_tensor=audio_tensor,
                sample_rate=sample_rate,
                output_sample_rate=request.sample_rate,
                response_format=request.response_format or "wav",
                speed=self._audio_encode_speed(request),
                base64_encode=base64_encode,
            )
""",
    """            audio_duration_s = None
            if sample_rate > 0:
                audio_array = np.asarray(audio_tensor)
                audio_channels = _infer_audio_num_channels(audio_array)
                audio_duration_s = audio_array.size / max(audio_channels, 1) / sample_rate
                if raw_request is not None and audio_duration_s > 0:
                    raw_request.state.audio_duration_s = audio_duration_s

            audio_obj = CreateAudio(
                audio_tensor=audio_tensor,
                sample_rate=sample_rate,
                output_sample_rate=request.sample_rate,
                response_format=request.response_format or "wav",
                speed=self._audio_encode_speed(request),
                base64_encode=base64_encode,
            )
""",
)

replace_once(
    serving_path,
    """                audio_bytes, media_type = await self._generate_audio_bytes(
                    request,
                    request_id=request_id,
                    usage_out=usage_box,
                    collect=collect,
                    request_arrival_ts=request_arrival_ts,
                )
""",
    """                audio_bytes, media_type = await self._generate_audio_bytes(
                    request,
                    request_id=request_id,
                    usage_out=usage_box,
                    collect=collect,
                    request_arrival_ts=request_arrival_ts,
                    raw_request=raw_request,
                )
""",
)

replace_once(
    serving_path,
    """                audio_bytes, media_type = await self._generate_audio_bytes(
                    retry_request,
                    request_id=retry_request_id,
                    usage_out=usage_box,
                    collect=collect,
                    request_arrival_ts=request_arrival_ts,
                )
""",
    """                audio_bytes, media_type = await self._generate_audio_bytes(
                    retry_request,
                    request_id=retry_request_id,
                    usage_out=usage_box,
                    collect=collect,
                    request_arrival_ts=request_arrival_ts,
                    raw_request=raw_request,
                )
""",
)

replace_once(
    serving_path,
    """            return Response(content=audio_bytes, media_type=media_type, headers=headers)
""",
    """            duration_s = getattr(raw_request.state, "audio_duration_s", None) if raw_request is not None else None
            if duration_s is not None:
                headers["Audio-Duration-Seconds"] = f"{duration_s:.3f}"
            return Response(content=audio_bytes, media_type=media_type, headers=headers)
""",
)
