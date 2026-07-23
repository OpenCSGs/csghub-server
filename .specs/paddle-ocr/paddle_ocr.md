# PaddleOCR Integration

## Goal

Integrate PaddleOCR into CSGHub as a deployable inference runtime and expose it through AIGateway with a stable OCR API.

The target v1 design should support self-hosted PaddleOCR/PaddleX serving for model inference deployments. It should not depend on PaddleOCR's hosted official API as the primary path, because hosted API usage does not run local inference inside our runtime framework.

## Sources

- PaddleOCR general OCR pipeline usage: https://www.paddleocr.ai/main/en/version3.x/pipeline_usage/OCR.html
- PaddleOCR self-hosted serving: https://www.paddleocr.ai/main/en/version3.x/inference_deployment/serving/serving.html
- PaddleOCR official API Go SDK: https://www.paddleocr.ai/latest/en/version3.x/inference_deployment/serving/paddleocr_official_api/go.html
- PaddleOCR official API CLI: https://www.paddleocr.ai/latest/en/version3.x/inference_deployment/serving/paddleocr_official_api/cli.html
- Legacy PaddleOCR HubServing docs: https://github.com/PaddlePaddle/PaddleOCR/blob/main/deploy/hubserving/readme_en.md
- LiteLLM OCR endpoint: https://docs.litellm.ai/docs/ocr
- Mistral OCR API: https://docs.mistral.ai (endpoint shape mirrored at https://docs.aimlapi.com/api-references/vision-models/ocr-optical-character-recognition/mistral-ai/mistral-ocr)
- OpenRouter PDF inputs: https://openrouter.ai/docs/guides/overview/multimodal/pdfs
- vLLM PaddleOCR-VL recipe: https://docs.vllm.ai/projects/recipes/en/stable/PaddlePaddle/PaddleOCR-VL.html
- new-api relay modes (no OCR): https://github.com/Calcium-Ion/new-api/blob/main/relay/constant/relay_mode.go

## Source Findings

PaddleOCR 3.x provides a general OCR pipeline for extracting editable text from images. Current documentation lists PP-OCRv3, PP-OCRv4, PP-OCRv5, and PP-OCRv6 support, with PP-OCRv6 as the newer default general OCR pipeline in recent docs.

The general OCR pipeline is composed of these modules:

- document image orientation classification, optional
- document image unwarping, optional
- text line orientation classification, optional
- text detection
- text recognition

PaddleOCR recommends PaddleX for self-hosted serving. The basic serving command is:

```bash
paddlex --serve --pipeline OCR
```

The service exposes an OCR endpoint, commonly shown as `/ocr`, and accepts a base64-encoded file payload. The basic serving mode is recommended for quick validation before evaluating high-stability serving based on Triton.

The PaddleOCR official API SDKs and CLI submit OCR or document parsing jobs to hosted PaddleOCR services. Those clients are useful as an external provider integration, but they do not run local models or load local inference artifacts inside CSGHub deployments.

Legacy PaddleOCR HubServing and PaddleServing docs exist, but for a new integration the PaddleX 3.x serving path is the better baseline because it aligns with current PaddleOCR documentation and PaddlePaddle 3.x support.

PaddleOCR also ships PaddleOCR-VL, a 0.9B vision-language document parsing model. vLLM supports it natively, so a standard vLLM deployment exposes it through the OpenAI chat completions API with an `image_url` content part and a task prompt such as `OCR:`, `Table Recognition:`, `Formula Recognition:`, or `Chart Recognition:`. This means PaddleOCR-VL can be served through CSGHub's existing vLLM runtime and AIGateway chat completions route without any new gateway code. The classic PP-OCR pipeline (PaddleX serving) remains the path that needs the new `/v1/ocr` route, and it is the only path that returns per-line bounding boxes.

## AI Gateway Prior Art

A July 2026 survey of how other AI gateways expose OCR:

- **LiteLLM**: the only major gateway with a first-class OCR route. `POST /v1/ocr`, Mistral API-compatible. Providers are registered with `mode: ocr` in the model list. Supports Mistral OCR, Azure Document Intelligence, Azure AI Mistral, and Vertex AI OCR. Accepts JSON document references and multipart uploads.
- **Mistral OCR API**: the de-facto OCR API shape that LiteLLM adopted. JSON body with `document: {type: document_url|image_url, document_url: <url or base64 data URI>}`. Response is `pages[]` with per-page `markdown`, extracted `images` with bounding boxes, page `dimensions`, and `usage_info.pages_processed`. Billed per page.
- **OpenRouter**: no OCR route. PDF parsing is a `file-parser` plugin on chat completions with selectable engines (`mistral-ocr` at roughly $2 per 1,000 pages, `cloudflare-ai` free, or the model's native file support billed as input tokens).
- **new-api / one-api**: no OCR relay mode. The relay layer covers chat, completions, embeddings, moderations, images, Midjourney, audio speech/transcription/translation, Suno, video, rerank, responses, realtime, and Gemini passthrough. OCR workloads run as vision-model chat completions.
- **Portkey**: no OCR or document parsing support.

Two conclusions for CSGHub:

- A first-class `/v1/ocr` route is validated by LiteLLM and Mistral rather than being an outlier, and per-page billing matches industry practice.
- Gateways that skip a dedicated route still serve OCR through chat completions with vision models. CSGHub gets that path for free once PaddleOCR-VL is deployable through vLLM, so both paths should be supported.

## Current CSGHub Findings

AIGateway currently exposes OpenAI-style API families:

- `GET /v1/models`
- `POST /v1/responses`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- `POST /v1/rerank`
- `POST /v1/images/generations`
- `POST /v1/images/edits`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/speech`
- `POST /v1/videos`

There is no existing OCR route.

Runtime frameworks are loaded from `configs/inference/*.json`. `component/runtime_architecture.go` reads each JSON file, unmarshals it into `types.EngineConfig`, stores a `runtime_framework`, and stores supported architectures and supported model names. A PaddleOCR runtime can therefore be added by creating a normal inference framework config.

AIGateway model listing already includes running deploy metadata from deploy records:

- public model ID
- `task`
- `metadata.llm_type`
- `metadata.repo_path`
- `runtime_framework`
- `endpoint`

This means OCR deployments can appear in `/v1/models` with the same model discovery path as existing inference models if the deploy task has an OCR task value and the runtime framework is seeded.

The closest existing AIGateway handler pattern is audio transcription:

- parse multipart form
- require `model` and `file`
- resolve model target
- start modal trace
- check balance
- rewrite the model field to the upstream model name
- proxy to the runtime endpoint
- wrap response
- record usage asynchronously

OCR should follow this shape rather than introducing handler-local special cases outside the normal AIGateway flow.

## Design Decisions

- Add PaddleOCR as a self-hosted inference runtime framework.
- Add a first-class AIGateway OCR route instead of overloading chat, responses, image generation, or audio transcription.
- Use PaddleX basic serving as the v1 runtime baseline.
- Normalize PaddleX OCR responses into an AIGateway-owned OCR response shape.
- Keep the raw upstream OCR result available in the response for compatibility and debugging.
- Add a dedicated OCR pipeline task value so `/v1/models?task=...` can filter OCR-capable deployments.
- Meter OCR by request/page/image count in v1. Do not reuse text-token or audio-duration accounting.
- Treat PaddleOCR official hosted API as a possible future external provider path, not the default CSGHub inference integration.
- Support two PaddleOCR deployment modes: the classic PP-OCR pipeline through PaddleX serving behind the new `/v1/ocr` route, and PaddleOCR-VL through the existing vLLM runtime and chat completions route (no new gateway code).
- Keep multipart as the v1 request format (parity with audio transcription), and plan a Mistral-compatible JSON body (`document` with URL or base64 data URI) as a fast-follow so clients using the Mistral OCR SDK shape work against AIGateway.
- Reserve an optional per-page `markdown` field in the OCR response for VL-backed or layout-parsing upstreams; the classic pipeline leaves it empty.
- Do not update GitLab issues or MRs as part of this spec.

## Public API

### Route

```text
POST /v1/ocr
```

The route should require the same API-key authentication as other AIGateway inference routes.

### Multipart Request

`multipart/form-data` is the v1 request format.

A Mistral-compatible JSON variant is planned as a fast-follow (not v1): `application/json` with `model` and `document: {type: "document_url"|"image_url", document_url: "<url or base64 data URI>"}`. It maps onto the same upstream transform, since PaddleX serving consumes base64 file bytes either way.

Required fields:

- `model`: public AIGateway model ID
- `file`: image or PDF file

Optional fields:

- `page_ranges`: page range for PDF or multi-page input, for example `1,3-5`
- `use_doc_orientation_classify`: boolean
- `use_doc_unwarping`: boolean
- `use_textline_orientation`: boolean
- `return_image`: boolean, defaults to false
- `raw_response`: boolean, defaults to false

Allowed file content should include common OCR inputs:

- `image/png`
- `image/jpeg`
- `image/webp`
- `image/bmp`
- `image/tiff`
- `application/pdf`

### Example Request

```bash
curl https://hub.example.com/v1/ocr \
  -H "Authorization: Bearer $CSGHUB_API_KEY" \
  -F "model=owner/paddleocr-demo:abc123" \
  -F "file=@invoice.png" \
  -F "use_doc_orientation_classify=true" \
  -F "use_doc_unwarping=false" \
  -F "use_textline_orientation=true"
```

### Response

```json
{
  "id": "ocr_abc123",
  "object": "ocr.result",
  "created": 1760000000,
  "model": "owner/paddleocr-demo:abc123",
  "text": "recognized full text",
  "pages": [
    {
      "index": 0,
      "text": "recognized page text",
      "markdown": "",
      "lines": [
        {
          "text": "line text",
          "score": 0.98,
          "bbox": [[10, 20], [200, 20], [200, 40], [10, 40]]
        }
      ],
      "image_url": ""
    }
  ],
  "usage": {
    "pages": 1,
    "images": 1
  },
  "raw_result": {}
}
```

`raw_result` should be omitted unless `raw_response=true` or an internal/debug option enables it.

`markdown` is reserved for VL-backed or layout-parsing upstreams (for example PaddleOCR-VL or PP-StructureV3 output) and is omitted or empty for the classic PP-OCR pipeline.

## Runtime Framework

Add `configs/inference/paddleocr.json`.

Initial shape:

```json
{
  "engine_name": "paddleocr",
  "enabled": 1,
  "container_port": 8000,
  "model_format": "paddle_static",
  "description": "PaddleOCR/PaddleX self-hosted OCR serving runtime",
  "engine_images": [
    {
      "compute_type": "gpu",
      "image": "opencsghq/paddleocr:3.7.0",
      "driver_version": "12.8",
      "engine_version": "3.7.0"
    },
    {
      "compute_type": "cpu",
      "image": "opencsghq/paddleocr-cpu:3.7.0",
      "engine_version": "3.7.0"
    }
  ],
  "supported_archs": [
    "PaddleOCRPipeline",
    "OCR"
  ],
  "supported_models": [
    "PP-OCRv6",
    "PP-OCRv5",
    "PP-OCRv5-latin"
  ]
}
```

Image tags are pinned to PaddleOCR `3.7.0` (with PaddleX `3.7.x` and PaddlePaddle `3.3.x` underneath). Revalidate against the latest PaddleOCR release before publishing new image tags.

### Runtime Image

The runtime image downloads the CSGHub model repo via `entry.py`, then starts a PaddleX serving process:

```bash
paddlex --serve --pipeline "${MODEL_DIR}/pipeline.yaml" \
  --host 0.0.0.0 \
  --port "${PORT:-8000}" \
  --device "${PADDLEOCR_DEVICE:-gpu:0}"
```

**Model source strategy (default: `HF_ENDPOINT` first, official-hoster fallback).** Official model repos on HF/ModelScope/OpenCSG (e.g. `PP-OCRv6_medium_rec`) ship only weights for a single sub-model — an OCR pipeline needs det + rec + optional classifiers, which PaddleX resolves by `model_name` at runtime. `serve.sh` points PaddleX's HuggingFace hoster at the deployment-provided `HF_ENDPOINT` (`PADDLE_PDX_HUGGING_FACE_ENDPOINT=${HF_ENDPOINT}/hf` — CSGHub serves its HF-compatible API under the `/hf` subpath — and `HF_TOKEN=$ACCESS_TOKEN`), so sub-models are pulled from that hub first; missing ones fall through PaddleX's built-in hoster chain (AIStudio → ModelScope → BOS). When the downloaded repo contains `pipeline.yaml`/`OCR.yaml`, that config is used as-is. When it is a single recognition model (has `inference.pdiparams`, repo basename ends `_rec`), `serve.sh` runs `paddlex --get_pipeline_config OCR` and patches it via `gen_pipeline.py`: `TextRecognition` is pointed at the local weights (`model_dir` = the downloaded repo), and `TextDetection` is switched to the matching-tier det model name (language prefixes `latin_`/`korean_`/`eslav_` are stripped for the det name) which is still resolved from the model sources — so deploying `PP-OCRv6_tiny_rec` actually serves the tiny tier instead of the medium default. Otherwise the built-in `OCR` pipeline is served and every sub-model is resolved from the configured sources. The runtime images pin `huggingface-hub==0.36.2` because hf_hub 1.x lists repo files via the `/tree` API, which CSGHub's `/hf` endpoint does not implement — 0.x uses `model_info` + `siblings`, which works.

For air-gapped deployments, `PADDLEOCR_MODEL_SOURCE=local-only` restores the strict offline mode: `PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True`, no external sources, and the repo **must** be a self-contained bundle (`pipeline.yaml` whose `model_dir` entries point at local subdirs) — startup fails fast otherwise. Bundle layout:

```
<repo>/
├── pipeline.yaml                 # every model_dir points at a local subdir
├── PP-OCRv6_medium_det/          # inference.pdiparams, inference.yml, ...
├── PP-OCRv6_medium_rec/
├── PP-LCNet_x1_0_doc_ori/        # optional: doc orientation classify
├── PP-LCNet_x1_0_textline_ori/   # optional: textline orientation
└── UVDoc/                        # optional: doc unwarping
```

A minimal bundle with only det + rec is valid if `use_doc_preprocessor`/`use_textline_orientation` are `False` in the pipeline config. The template comes from `paddlex --get_pipeline_config OCR --save_path pipeline.yaml` with each `model_dir: null` rewritten to the local subdir.

Environment variables:

- `PADDLEOCR_MODEL_SOURCE`: `hub` (default, pull from `HF_ENDPOINT` first with official-hoster fallback) or `local-only` (strict offline, requires pipeline bundle)
- `PADDLEX_PIPELINE`: explicit pipeline config path override
- `PADDLEOCR_DEVICE`: `gpu`, `cpu`, or another PaddleX-supported device (set per image)
- `PORT`: serving port, defaults to `8000`
- `ENGINE_ARGS`: extra args appended to `paddlex --serve` (e.g. `--use_hpip`)
- `PADDLEOCR_RETURN_URLS`: future option for binary result URLs

ROCm/AMD note: no `paddleocr-rocm` image is planned for now. The repo's ROCm images standardize on ROCm 7.2.2 (`rocm/pytorch:rocm7.2.2_ubuntu22.04_py3.10_pytorch_release_2.10.0`), but PaddlePaddle does not publish official ROCm pip wheels — its AMD support ships mainly through Baidu's DCU (Hygon) custom builds, so a ROCm variant would need a different base image and separate validation.

### Runtime Endpoint

The PaddleX service endpoint should be treated as the upstream runtime API. For v1, AIGateway can transform its multipart request into the PaddleX expected JSON body:

```json
{
  "file": "<base64 file bytes>",
  "fileType": 1
}
```

The handler or adapter should derive `fileType` from the uploaded input:

- image: `1`
- PDF: value to be confirmed against the exact PaddleX serving contract during runtime validation

If the PaddleX endpoint path differs from `/ocr` in the selected image version, the runtime image should normalize it or the AIGateway adapter should make the endpoint path configurable.

## AIGateway Design

### Route

Add the route in `aigateway/router/aigateway.go`:

```go
v1Group.POST("/ocr", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.OCR)
```

Use the modal API rate limiter because OCR is image/document inference and can be resource intensive.

### Types

Add `aigateway/types/ocr.go`.

Recommended DTOs:

```go
type OCRRequest struct {
    Model                       string
    PageRanges                  string
    UseDocOrientationClassify   *bool
    UseDocUnwarping             *bool
    UseTextlineOrientation      *bool
    ReturnImage                 bool
    RawResponse                 bool
}

type OCRResponse struct {
    ID        string       `json:"id"`
    Object    string       `json:"object"`
    Created   int64        `json:"created"`
    Model     string       `json:"model"`
    Text      string       `json:"text"`
    Pages     []OCRPage    `json:"pages"`
    Usage     OCRUsage     `json:"usage"`
    RawResult any          `json:"raw_result,omitempty"`
}

type OCRPage struct {
    Index    int       `json:"index"`
    Text     string    `json:"text"`
    Markdown string    `json:"markdown,omitempty"`
    Lines    []OCRLine `json:"lines"`
    ImageURL string    `json:"image_url,omitempty"`
}

type OCRLine struct {
    Text  string      `json:"text"`
    Score *float64    `json:"score,omitempty"`
    BBox  interface{} `json:"bbox,omitempty"`
}

type OCRUsage struct {
    Pages  int `json:"pages"`
    Images int `json:"images"`
}
```

The handler package can own multipart parsing, but the response and normalized request data should live in `aigateway/types`.

### Handler Flow

Add `aigateway/handler/openai_ocr.go`.

Flow:

1. Parse `multipart/form-data`.
2. Require `model`.
3. Require exactly one `file`.
4. Validate uploaded content type and size.
5. Resolve the target model through `resolveModelTarget`.
6. Ensure the resolved model is OCR-capable:
   - `model.Task == "optical-character-recognition"`, or
   - `model.RuntimeFramework == "paddleocr"`
7. Start modal trace with output type `text`.
8. Run balance check.
9. Transform multipart upload into the PaddleX OCR serving payload.
10. Apply model auth headers.
11. Proxy to the PaddleOCR runtime endpoint.
12. Parse PaddleX response.
13. Normalize to `types.OCRResponse`.
14. Record OCR usage asynchronously.

The handler should follow the existing transcription handler pattern for target resolution, trace lifecycle, balance handling, auth header application, reverse proxy creation, and async usage recording.

### Adapter

Add an OCR adapter package only if more than one upstream protocol is expected:

```text
aigateway/component/adapter/ocr
  adapter.go
  paddlex.go
```

For v1, a PaddleX adapter is enough:

- build upstream request from multipart input
- select endpoint path, default `/ocr`
- parse PaddleX response
- normalize pages, lines, text, image output, and raw result

If the runtime image can expose an AIGateway-native `/v1/ocr` contract directly, the adapter can be simpler and only preserve the existing proxy/rewrite pattern.

### Task Type

Add a new common pipeline task:

```go
OpticalCharacterRecognition PipelineTask = "optical-character-recognition"
```

This lets deployed PaddleOCR models appear with:

```json
{
  "task": "optical-character-recognition"
}
```

Then clients can discover OCR models with:

```text
GET /v1/models?task=optical-character-recognition
```

### Model Listing

No separate AIGateway model-list path is needed. Existing CSGHub deploys already populate model fields from running deploy records.

The important requirement is that OCR deploy records carry:

- `Task = optical-character-recognition`
- `RuntimeFramework = paddleocr`
- `Endpoint` pointing to the PaddleOCR serving endpoint

### Usage and Billing

Add OCR usage semantics instead of pretending OCR uses text tokens or audio duration.

This matches industry practice: Mistral OCR bills per processed page (`usage_info.pages_processed`), and OpenRouter's Mistral OCR engine is priced per 1,000 pages.

Initial v1 usage dimensions:

- request count
- page count
- image count

If the accounting SKU system needs a single unit, use page count for PDFs and image count for image files. For single-image requests, both may be `1`.

If the runtime cannot reliably report pages, derive:

- image upload: `pages = 1`, `images = 1`
- PDF upload: page count from runtime response, if present; otherwise `pages = 1` as a conservative fallback with a warning log

### Trace and Logs

Trace metadata should include:

- `aigateway.api = /v1/ocr`
- `aigateway.model.id`
- `aigateway.ocr.page_ranges`
- `aigateway.ocr.return_image`
- `aigateway.ocr.pages`
- `aigateway.ocr.images`

Do not store raw OCR text in span attributes by default.

## Error Handling

Return OpenAI-style AIGateway errors consistent with existing handlers.

Recommended errors:

- missing `model`: `invalid_request_error`
- missing `file`: `invalid_request_error`
- multiple files: `invalid_request_error`
- unsupported content type: `invalid_request_error`
- model not found: existing model target error handling
- model is not OCR-capable: `unsupported_model`
- upstream unavailable: existing upstream unavailable error handling
- malformed PaddleX response: `upstream_response_error`

## Security

- Require API-key auth.
- Apply existing model target authorization and visibility rules.
- Do not log file content or recognized OCR text by default.
- Enforce upload size limits before reading the full file into memory.
- Avoid returning raw upstream payloads unless explicitly requested.
- If `return_image` produces binary output, prefer storing generated artifacts in object storage and returning URLs instead of inline base64 for large results.

## Rollout Plan

### Phase 1: Runtime Validation

- Build CPU and GPU PaddleOCR runtime images.
- Run `paddlex --serve --pipeline OCR` in the image.
- Confirm exact request and response contract for image input and PDF input.
- Confirm endpoint path and `fileType` values.
- Pin image tags.
- Optionally validate PaddleOCR-VL serving through vLLM (`vllm serve PaddlePaddle/PaddleOCR-VL`) as the zero-gateway-code companion path.

### Phase 2: Runtime Framework Registration

- Add `configs/inference/paddleocr.json`.
- Run runtime framework scan or startup initialization.
- Confirm `runtime_framework` and `runtime_architectures` rows are created.
- Confirm OCR model repositories can select PaddleOCR as an inference runtime.

### Phase 3: AIGateway API

- Add `POST /v1/ocr`.
- Add OCR request/response types.
- Add PaddleX request transform and response normalization.
- Add OCR-specific usage counter.
- Add trace metadata.

### Phase 4: Tests and Docs

- Add handler tests for multipart validation, model resolution, request transform, and response normalization.
- Add adapter tests for PaddleX response parsing.
- Add model-list test coverage for `optical-character-recognition`.
- Add a user-facing usage guide if portal or docs need API examples.

## Test Plan

Unit tests:

- `aigateway/types` OCR response JSON shape
- OCR multipart parser rejects missing model, missing file, multiple files, unsupported content type
- OCR handler rewrites public model ID to runtime model name
- PaddleX adapter builds expected upstream JSON
- PaddleX adapter normalizes page and line results
- OCR usage is derived from normalized response

Focused package tests:

```bash
go test ./aigateway/types ./aigateway/handler ./aigateway/component/adapter/ocr
```

Runtime validation:

```bash
curl http://localhost:8000/ocr \
  -H "Content-Type: application/json" \
  -d '{"file":"<base64>","fileType":1}'
```

AIGateway validation:

```bash
curl http://localhost:8080/v1/ocr \
  -H "Authorization: Bearer $CSGHUB_API_KEY" \
  -F "model=<paddleocr-model-id>" \
  -F "file=@demo.png"
```

## Open Questions

- Exact PaddleX `fileType` values for PDF and multi-page inputs need runtime validation.
- Should v1 support PDF uploads, or should it ship image-only first and add PDF after page-count billing is verified?
- Should the runtime image normalize PaddleX output into AIGateway's OCR response shape directly, or should AIGateway own all normalization?
- What is the final SKU unit for OCR: request, page, image, or weighted page by file type? Industry precedent (Mistral per-page, OpenRouter per-1,000-pages) favors page count.
- Does portal need a dedicated OCR playground, or is API-only enough for the first milestone?
- Mistral-compatible JSON request body: v1 or fast-follow? Current recommendation is fast-follow, confirmed before implementation starts.

Resolved during gateway prior-art research:

- First-class `/v1/ocr` route vs chat-completions-only: keep the first-class route for the PaddleX pipeline (validated by LiteLLM/Mistral), and additionally serve PaddleOCR-VL through the existing vLLM chat completions path.

## Recommended V1 Scope

Ship the smallest complete integration:

- PaddleOCR runtime image using PaddleX basic serving
- `configs/inference/paddleocr.json`
- `optical-character-recognition` task value
- `POST /v1/ocr` multipart image upload
- normalized text/pages/lines response
- request/image-count usage
- tests for handler validation and adapter normalization

Companion path that ships independently with no new gateway code:

- PaddleOCR-VL deployed through the existing vLLM runtime framework, callable through `POST /v1/chat/completions` with `image_url` input and an `OCR:` task prompt

Defer:

- Mistral-compatible JSON request body (fast-follow)
- official hosted PaddleOCR API provider
- Triton/high-stability serving
- PDF-specific billing if page count is not reliable
- object-storage URLs for result images
- portal playground
