# LongCat Video Avatar runtime

This runtime wraps the official LongCat-Video Avatar scripts with the
asynchronous LightX2V task API. Tasks execute serially to avoid GPU memory
contention. `GPU_NUM` controls both `torchrun --nproc_per_node` and context
parallelism. `LongCat-Video-Avatar` selects `avatar-v1.0`;
`LongCat-Video-Avatar-1.5` selects `avatar-v1.5` and enables distillation and
INT8.

## Configuration

Required in a CSGHub deployment:

- `HF_ENDPOINT`: CSGHub endpoint.
- `ACCESS_TOKEN`: CSGHub access token when the repositories are private.
- `REPO_ID`: Avatar repository passed by the deployment.

Optional settings include `REVISION`, `BASE_MODEL_REVISION`, `GPU_NUM` (default
1), `MAX_FILE_BYTES` (default 100 MiB), `WORKSPACE` (default `/workspace`),
and `BASE_MODEL_ID`. The base model defaults to `LongCat-Video` in the same
namespace as `REPO_ID`. Each repository is downloaded under
`/workspace/<repo_id>`, matching the other inference runtimes. Override
`BASE_MODEL_ID` only when the base checkpoint lives in another repository.

## API

Submit one audio file for AT2V, or include `image_file` for AI2V. Submit two
repeated `audio` fields plus an image for multi-person generation:

```shell
curl -X POST http://127.0.0.1:8000/v1/tasks/video/form \
  -F 'prompt=Two people speak in a recording studio' \
  -F 'audio=@person1.wav;type=audio/wav' \
  -F 'audio=@person2.wav;type=audio/wav' \
  -F 'image_file=@people.png;type=image/png' \
  -F 'audio_type=para' \
  -F 'resolution=480p' \
  -F 'num_segments=1' \
  -F 'bbox={"person1":[40,20,440,400],"person2":[40,430,440,810]}'
```

`audio_file` is accepted as an alias for `audio`. `audio_type=para` overlays
equal-duration clips; `add` concatenates them. Supported dimensions are
832x480 and 1280x768, supplied either as `resolution=480p|720p` or matching
`width` and `height`. `ref_img_index` and `mask_frame_range` are optional.

Poll and download with:

```shell
curl http://127.0.0.1:8000/v1/tasks/TASK_ID/status
curl -o result.mp4 \
  http://127.0.0.1:8000/v1/files/download/outputs/videos/TASK_ID.mp4
curl http://127.0.0.1:8000/health
```

Status values are `submitted`, `running`, `succeed`, and `failed`.

The CUDA image follows the upstream reference environment. The ROCm image is
an experimental port using the required ROCm 7.2.2 PyTorch base and a pinned
ROCm FlashAttention build; validate BF16, INT8, RCCL, memory use, and output
quality on the target AMD GPU before production rollout.
