# CSGHUB Inference Images Building

## Login Container Registry
```bash
OPENCSG_ACR="opencsg-registry.cn-beijing.cr.aliyuncs.com"
OPENCSG_ACR_USERNAME=""
OPENCSG_ACR_PASSWORD=""
echo "$OPENCSG_ACR_PASSWORD" | docker login $OPENCSG_ACR -u $OPENCSG_ACR_USERNAME --password-stdin
```

## Build Multi-Platform Images
```bash
export BUILDX_NO_DEFAULT_ATTESTATIONS=1

# For vllm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/vllm:v0.24.0
export IMAGE_TAG=v0.24.0
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/vllm:${IMAGE_TAG} \
  -f Dockerfile.vllm \
  --push .

# For amd-vllm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/amd-vllm:rocm7.2.1_vllm_0.24.0
export IMAGE_TAG=rocm7.2.1_vllm_0.24.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/amd-vllm:${IMAGE_TAG} \
  -f Dockerfile.vllm-amd \
  --push .
  
# For vllm cpu only: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/vllm-cpu:v0.24.0
export IMAGE_TAG=v0.24.0
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/vllm-cpu:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/vllm-cpu:latest \
  -f Dockerfile.vllm-cpu \
  --push .

# For tgi: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/tgi:3.2
export IMAGE_TAG=3.2
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/tgi:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/tgi:latest \
  -f Dockerfile.tgi \
  --push .

# For TGI ROCm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/tgi-rocm:rocm6.3.1_tgi_3.3.6
export IMAGE_TAG=rocm6.3.1_tgi_3.3.6
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/tgi-rocm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/tgi-rocm:latest \
  -f Dockerfile.tgi-rocm \
  --push .

# For sglang: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/sglang:v0.5.14-cu130
export IMAGE_TAG=v0.5.14-cu130
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/sglang:${IMAGE_TAG} \
  -f Dockerfile.sglang \
  --push .

# For nvidia-vllm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/vllm-nvidia:25.11-py3
export IMAGE_TAG=25.11-py3
docker buildx build --platform linux/amd64,linux/arm64 \
   -t ${OPENCSG_ACR}/opencsghq/nvidia-vllm:${IMAGE_TAG} \
  -f Dockerfile.vllm-nvidia \
  --push .

# For mindie: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/mindie:2.0-csg-1.0.RC2
export IMAGE_TAG=2.0-csg-1.0.RC2
docker buildx build --platform linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/mindie:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/mindie:latest \
  -f Dockerfile.mindie \
  --push .

# For hf-inference-toolkit: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/hf-inference-toolkit:0.5.3
export IMAGE_TAG=0.5.3
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/hf-inference-toolkit:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/hf-inference-toolkit:latest \
  -f Dockerfile.hf-inference-toolkit \
  --push .
# For FunASR CUDA: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/funasr:cuda12.8
export IMAGE_TAG=cuda12.8
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/funasr:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/funasr:latest \
  -f Dockerfile.funasr \
  --push .
# For FunASR ROCm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/funasr-rocm:rocm7.2.2
export IMAGE_TAG=rocm7.2.2
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/funasr-rocm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/funasr-rocm:latest \
  -f Dockerfile.funasr-rocm \
  --push .
# For FunASR CPU: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/funasr-cpu:latest
export IMAGE_TAG=latest
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/funasr-cpu:${IMAGE_TAG} \
  -f Dockerfile.funasr-cpu \
  --push .
# For PaddleOCR CUDA: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/paddleocr:3.7.0
export IMAGE_TAG=3.7.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/paddleocr:${IMAGE_TAG} \
  --target paddleocr \
  -f Dockerfile.paddleocr \
  --push .
# For PaddleOCR-VL CUDA: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/paddleocr-vl:3.7.0
export IMAGE_TAG=3.7.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/paddleocr-vl:${IMAGE_TAG} \
  --target paddleocr-vl \
  -f Dockerfile.paddleocr \
  --push .
# For PaddleOCR CPU: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/paddleocr-cpu:3.7.0
# (paddlepaddle pinned to 3.2.2: 3.3.1 crashes on CPU in the oneDNN path, PaddleOCR issue #18162)
export IMAGE_TAG=3.7.0
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/paddleocr-cpu:${IMAGE_TAG} \
  -f Dockerfile.paddleocr-cpu \
  --push .
# For Diffusers image generation and editing: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/diffusers:0.39.0
export IMAGE_TAG=0.39.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/diffusers:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/diffusers:latest \
  -f Dockerfile.diffusers \
  --push .
# For Diffusers image generation and editing ROCm: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/diffusers-rocm:0.39.0
export IMAGE_TAG=0.39.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/diffusers-rocm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/diffusers-rocm:latest \
  -f Dockerfile.diffusers-rocm \
  --push .
# For Text Embeddings Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/tei:cpu-1.6
export IMAGE_TAG=cpu-1.6
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/tei:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/tei:latest \
  -f Dockerfile.tei-cpu \
  --push .
# For Text Embeddings Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/tei:1.6
export IMAGE_TAG=1.6
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/tei:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/tei:latest \
  -f Dockerfile.tei \
  --push .
# For Text Llama.cpp Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/llama.cpp:b5215
export IMAGE_TAG=b5215
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/llama.cpp:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/llama.cpp:latest \
  -f Dockerfile.llama.cpp \
  --push .
# For Text Llama.cpp ROCm Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/llama.cpp-rocm:rocm7.2.2-b9787
export IMAGE_TAG=rocm7.2.2-b9787
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/llama.cpp-rocm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/llama.cpp-rocm:latest \
  -f Dockerfile.llama.cpp-rocm \
  --push .
# For Text ktransformers Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/ktransformers:0.2.1.post1  
export IMAGE_TAG=0.2.3
docker build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/ktransformers:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/ktransformers:latest \
  -f Dockerfile.ktransformers \
  --push .
# For iFLYTEK AudioFly text-to-audio: opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/audiofly:1.0
export IMAGE_TAG=1.0
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/audiofly:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/audiofly:latest \
  -f Dockerfile.audiofly \
  --push .
# AMD ROCm variant
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/audiofly-rocm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/audiofly-rocm:latest \
  -f Dockerfile.audiofly-rocm \
  --push .

# LongCat Video Avatar models (CUDA 12.4 and ROCm 7.2.2)
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/longcat-video:1.5-cu124 \
  -f Dockerfile.longcat-video --push .
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/longcat-video-rocm:1.5-rocm7.2.2 \
  -f Dockerfile.longcat-video-rocm --push .
```
*Note: The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time.*

## Run Inference Image Locally
```bash
# Run VLLM
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  --gpus device=1 \
  -p 8000:8000 \
  ${OPENCSG_ACR}/opencsghq/vllm:v0.24.0

# Run TGI
docker run -d \
  -e ACCESS_TOKEN=xxx  \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -v llm:/workspace \
  --gpus device=7 \
  -p 8000:8000
  ${OPENCSG_ACR}/opencsghq/tgi:2.2

# Run TGI on AMD ROCm (the image supports gfx90a and gfx942)
docker run -d \
  --device=/dev/kfd \
  --device=/dev/dri \
  --group-add video \
  --ipc=host \
  --shm-size 256g \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -v llm-rocm:/workspace \
  -p 8000:8000 \
  ${OPENCSG_ACR}/opencsghq/tgi-rocm:rocm6.3.1_tgi_3.3.6

# Run MINDIE
docker run -d \
  -e ACCESS_TOKEN=xxx  \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -v llm:/workspace \
  --gpus device=7 \
  -p 8000:8000
  ${OPENCSG_ACR}/opencsghq/mindie:1.8-csg-1.0.RC2

# Run FunASR CPU with a CSGHub model
docker run --rm -it \
  --name funasr-cpu-test \
  -p 8000:8000 \
  -e REPO_ID="AIWizards/FunAudioLLM_Fun-ASR-Nano-2512" \
  -e REVISION="master" \
  -e ACCESS_TOKEN="xxx" \
  -e HF_ENDPOINT="https://opencsg-stg.com" \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/funasr-cpu:latest

# Call FunASR OpenAI-compatible transcription API
curl --max-time 600 -X POST http://127.0.0.1:8000/v1/audio/transcriptions \
  -F "file=@/path/to/audio.mp3" \
  -F "model=local" \
  -F "response_format=text"

# Call FunASR OpenAI-compatible translation API (any language -> English).
# Only works when REPO_ID is a multilingual Whisper model (e.g. openai/whisper-large-v3);
# turbo, English-only (.en) checkpoints (e.g. openai/whisper-small.en), and non-Whisper
# models (Fun-ASR, SenseVoice, Paraformer, Qwen3-ASR) return 501. Translation is passed
# to the underlying Whisper model via FunASR's DecodingOptions (task=translate).
curl --max-time 600 -X POST http://127.0.0.1:8000/v1/audio/translations \
  -F "file=@/path/to/audio.mp3" \
  -F "model=local" \
  -F "response_format=text"

# Run image editing with a CSGHub Diffusers model
docker run -d \
  --name diffusers-test \
  --gpus device=0 \
  -p 8000:8000 \
  -e REPO_ID="Qwen/Qwen-Image-Edit" \
  -e REVISION="main" \
  -e ACCESS_TOKEN="xxx" \
  -e HF_ENDPOINT="https://hub.opencsg.com" \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/diffusers:0.39.0

# Call OpenAI-compatible image edit API
curl --max-time 600 -X POST http://127.0.0.1:8000/v1/images/edits \
  -F "model=local" \
  -F "prompt=make the sky sunset orange" \
  -F "image=@/path/to/input.png" \
  -F "response_format=b64_json"

# Run a text-to-speech model with vLLM-Omni (HF_TASK=text-to-speech switches
# single-node.sh to `vllm serve --omni`)
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="Qwen/Qwen3-TTS-12Hz-1.7B-CustomVoice" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -e HF_TASK=text-to-speech \
  --gpus device=0 \
  -p 8000:8000 \
  ${OPENCSG_ACR}/opencsghq/vllm:v0.24.0

# Call OpenAI-compatible speech API
curl --max-time 600 -X POST http://127.0.0.1:8000/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"input": "Hello, how are you?", "voice": "vivian", "language": "English"}' \
  --output output.wav

# Run iFLYTEK AudioFly (latent-diffusion text-to-audio, OpenAI speech API)
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="iflytek/AudioFly" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  --gpus device=0 \
  -p 8000:8000 \
  ${OPENCSG_ACR}/opencsghq/audiofly:1.0

# Call AudioFly with the same speech API (voice/speed are ignored; cfg and
# ddim_steps are optional AudioFly-specific parameters)
curl --max-time 600 -X POST http://127.0.0.1:8000/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"input": "Fierce winds howl through the valley", "cfg": 3.5, "ddim_steps": 200}' \
  --output output.wav

# Run a LongCat Video Avatar model. BASE_MODEL_ID and REPO_ID are downloaded
# separately because Avatar inference consumes both checkpoint repositories.
# BASE_MODEL_ID is inferred from the REPO_ID namespace unless overridden.
docker run --rm --gpus all -p 8000:8000 \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID=meituan-longcat/LongCat-Video-Avatar-1.5 \
  -e GPU_NUM=1 \
  -v longcat-models:/workspace \
  ${OPENCSG_ACR}/opencsghq/longcat-video:1.5-cu124

# ROCm uses the same API and environment variables.
docker run --rm --device=/dev/kfd --device=/dev/dri \
  --group-add video --ipc=host -p 8000:8000 \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -e ACCESS_TOKEN=xxx \
  -e GPU_NUM=1 \
  -v longcat-models-rocm:/workspace \
  ${OPENCSG_ACR}/opencsghq/longcat-video-rocm:1.5-rocm7.2.2

# Submit an asynchronous single-audio AT2V task. Add image_file for AI2V;
# repeat audio twice (and require image_file) for multi-person generation.
curl -X POST http://127.0.0.1:8000/v1/tasks/video/form \
  -F 'prompt=A presenter speaks naturally on a stage' \
  -F 'audio=@speech.wav;type=audio/wav' \
  -F 'resolution=480p'
curl http://127.0.0.1:8000/v1/tasks/TASK_ID/status
curl -o result.mp4 \
  http://127.0.0.1:8000/v1/files/download/outputs/videos/TASK_ID.mp4
```
*Note: HF_ENDPOINT should be use the real csghub address.*
*Note: FunASR downloads `REPO_ID` to `/workspace/${REPO_ID}` and preloads that local model at startup. The OpenAI-compatible `model` field can use `local`, the repo id, or the repo name.*
*Note: FunASR enables VAD chunking by default for long audio with `FUNASR_VAD_MODEL=fsmn-vad`, `FUNASR_VAD_MAX_SINGLE_SEGMENT_TIME=30000`, `FUNASR_BATCH_SIZE_S=60`, and `FUNASR_BATCH_SIZE_THRESHOLD_S=30`. Set `FUNASR_VAD_MODEL=none` to disable VAD.*
*Note: Diffusers image generation and editing uses a PyTorch CUDA 12.8 base image and downloads `REPO_ID` to `/workspace/${REPO_ID}` before loading the local Diffusers pipeline. The 0.39.0 image pins `diffusers==0.39.0` with `transformers==5.13.0`; the 0.38.0 runtime remains registered for models that do not require the new pipelines.*
*Note: vLLM text-to-speech is served by vLLM-Omni (`vllm serve --omni`) and exposes `POST /v1/audio/speech` plus `GET /v1/audio/voices`. Supported architectures (see `configs/inference/vllm.json` extra_archs): Qwen3-TTS, Fish Speech S2 Pro, Voxtral TTS, CosyVoice3, OmniVoice, VoxCPM2, MOSS-TTS-Nano.*
*Note: AudioFly is a latent-diffusion text-to-audio (sound effect) model, not an LLM-based TTS, so vLLM-Omni cannot serve it; the dedicated `audiofly` image wraps the model's own `ldm` inference code with the same OpenAI speech API (`/v1/audio/speech`, `/v1/audio/speech/batch`, `/v1/audio/voices`). Generation is non-streaming and wav-only; see `docker/inference/audiofly/README.md`.*

### TGI ROCm model compatibility

The `3.3.6-rocm` image has dedicated TGI routes for these model families:

- Text: Baichuan, Bloom, Cohere, DBRX, DeepSeek V2/V3, Falcon, Gemma 1/2/3 Text, GPT-2, GPT BigCode/SantaCoder, GPT-J, GPT-NeoX, Granite, Llama, Mamba, Mistral, Mixtral, MPT, OPT/Galactica, Phi/Phi-3/PhiMoE, Qwen2, StarCoder2, and T5.
- Multimodal: Gemma 3, Idefics 1/2/3, Llama 4, Llama 3.2 Vision (Mllama), Llava Next, PaliGemma, Qwen2-VL, and Qwen2.5-VL.

Other Transformers `AutoModelForCausalLM` and `AutoModelForSeq2SeqLM` architectures may use TGI's generic fallback, but are not registered in `amd-tgi.json` because optimized execution and sharding are not guaranteed.

ROCm-specific limits:

- The image is compiled for `gfx90a` and `gfx942` (MI210/MI250 and MI300; MI325X is architecture-compatible but not listed as tested upstream).
- Upstream TGI does not support AWQ checkpoints or a sliding-window attention kernel on ROCm. Mistral, Gemma 2, and Gemma 3 models must set `--max-input-tokens` no greater than the model's `sliding_window`.
- Gemma 2 passes an attention softcap that the ROCm paged-attention implementation rejects during paged decoding. Treat Gemma 2 and Gemma 3 as source-routed candidates that require ROCm smoke testing rather than default-stable models.
- The custom paged-attention fast path applies to FP16/BF16, head sizes 64 or 128, block sizes 16 or 32, GQA ratios 1 through 16, and contexts up to 131072 tokens; other shapes fall back to the bundled vLLM paged-attention kernel.
- AITER kernels are prebuilt only for `gfx942`; MoE models need additional validation on `gfx90a` devices.
- This CSGHub wrapper downloads safetensors weights only. Models available exclusively as PyTorch `.bin` files are not supported.

## inference image name, version and cuda version
| Task| Image Name | Version | CUDA Version | Fix
| --- | --- | --- | --- |--- |
|text generation / embedding / reranking / text to speech| vllm | v0.24.0 | 13.0 |pooling runner support for embedding and reranking; vllm-omni 0.24.0 for text-to-speech (/v1/audio/speech)|
|text generation / embedding / reranking| amd-vllm | rocm7.2.1_vllm_0.24.0 | - |ROCm 7.2.1, vLLM 0.24.0|
|text generation| vllm | v0.8.5 | 12.4 |fix hf hub timestamp|
|text generation| vllm-cpu | v0.24.0 | - |vLLM 0.24.0, supports --enable-prompt-tokens-details|
|text generation| tgi | 2.2 | 12.1 |- |
|text generation| tgi | 3.2 | 12.4 |fix hf hub timestamp|
|text generation| tgi-rocm | rocm6.3.1_tgi_3.3.6 | - |TGI 3.3.6, ROCm 6.3.1|
|image generation| hf-inference-toolkit | 0.5.3 | 12.1 |-|
|speech recognition| funasr | cuda12.8 | 12.8 |-|
|speech recognition| funasr-rocm | rocm7.2.2 | - |-|
|speech recognition| funasr-cpu | latest | - |-|
|image generation / editing| diffusers | 0.39.0 | 12.8 |diffusers 0.39.0, transformers 5.13.0|
|image generation / editing| diffusers | 0.38.0 | 12.8 |legacy runtime retained for existing pipelines|
|image generation / editing| diffusers-rocm | 0.39.0 | - |diffusers 0.39.0, transformers 5.13.0, ROCm 7.2.2|
|image generation / editing| diffusers-rocm | 0.38.0 | - |legacy runtime retained, ROCm 7.2.2|
|text generation| sglang | v0.5.14-cu130 | 13.0 |tool calling enabled by default|
|text generation| mindie | 2.0-csg-1.0.RC2 | 1.0.RC2 |- |
|text generation| llama.cpp | b5215 | - |- |
|text generation| llama.cpp-rocm | rocm7.2.2-b9787 | - |ROCm 7.2.2, llama.cpp b9787, official wide range ROCm targets|
|text generation| tei | 1.6 | - |- |
|text to audio| audiofly | 1.0 | 12.1 |iFLYTEK AudioFly LDM model with OpenAI speech API wrapper|
|text to audio| audiofly-rocm | 1.0 | - |iFLYTEK AudioFly LDM model, ROCm 7.2.2|
|audio-driven video| longcat-video | 1.5-cu124 | 12.4 |LongCat-Video-Avatar and 1.5 asynchronous runtime|
|audio-driven video| longcat-video-rocm | 1.5-rocm7.2.2 | - |LongCat-Video-Avatar and 1.5, experimental ROCm 7.2.2 port|


## API to Call Inference
```
curl -H "Content-type: application/json" -X POST -d '{
  "model": "OpenCSG/csg-wukong-1B",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is deep learning?"
    }
  ],
  "stream": true,
  "max_tokens": 20
}' http://127.0.0.1:8000/v1/chat/completions
```
*Note: VLLM and TGI has the same endpoint and request body.*

More reference for TGI: 
- [Text Generation Inference](https://huggingface.github.io/text-generation-inference/)
- [Text Generation Inference Messages API](https://huggingface.co/docs/text-generation-inference/en/messages_api)
