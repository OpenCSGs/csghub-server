# PaddleOCR-VL PaddleX Runtime Specification

## Status

This document defines the CSGHub runtime and AIGateway architecture for serving the PaddleOCR-VL series as full PaddleX document-parsing pipelines.

The classic OCR contract remains documented in [paddle_ocr.md](./paddle_ocr.md). The two runtime frameworks share an image lineage and public AIGateway route, but they intentionally expose different PaddleX pipelines and upstream protocols.

## Context

PaddleOCR-VL has two distinct serving modes:

1. Serve the VLM directly through an inference engine such as vLLM. This provides region recognition through an OpenAI-compatible model endpoint.
2. Serve the full PaddleX pipeline. This adds layout analysis, region cropping, reading order, recognition, and document-result assembly.

Direct vLLM support for `PaddleOCRVLForConditionalGeneration` does not make vLLM a document parser. Sending a full document directly to the VLM bypasses PaddleX layout analysis and assembly, which reduces quality for complex layouts, tables, formulas, and multi-page documents.

CSGHub therefore treats the two modes as different runtime choices:

| Runtime framework | Purpose | Public API | Runtime API |
| --- | --- | --- | --- |
| `vllm` | Direct VLM inference | `/v1/chat/completions` or `/v1/responses` | OpenAI-compatible model API |
| `paddleocr` | Classic text detection and recognition | `/v1/ocr` | `/ocr` |
| `paddleocr-vl` | Full document parsing | `/v1/ocr` | `/layout-parsing` |

## Goals

- Deploy PaddleOCR-VL as a complete PaddleX document-parsing pipeline.
- Support images and PDFs through the existing `POST /v1/ocr` AIGateway route.
- Preserve classic PaddleOCR behavior and its `/ocr` upstream protocol.
- Use the selected model repository for VL recognition while allowing PaddleX to resolve required submodels.
- Match each PaddleOCR-VL model version to its corresponding top-level PaddleX pipeline.
- Preserve provider-specific structured output behind `raw_response=true`.
- Keep pipeline selection explicit in the selected image's baked startup command.

## Non-goals

- Adding PaddleX document-parser steps to vLLM.
- Creating a second independently maintained Dockerfile for PaddleOCR-VL.
- Supporting PaddleOCR-VL on the CPU runtime without separate validation.
- Orchestrating a separate vLLM or SGLang recognition service in this version.
- Uploading PaddleX base64 result images to object storage.
- Flattening tables, formulas, and layout blocks into classic OCR line objects.
- Adding another public document-parsing route.

## Core Design Decisions

### Separate runtime framework

`paddleocr-vl` is a separate runtime framework rather than a mode hidden inside `paddleocr` or `vllm`.

This keeps these contracts explicit:

- runtime selection: `paddleocr-vl`;
- PaddleX pipeline: a version-matched PaddleOCR-VL pipeline;
- upstream endpoint: `/layout-parsing`;
- supported input: image and PDF;
- normalized result: page Markdown plus plain text.

The framework is registered by `configs/inference/paddleocr-vl.json` as a GPU-only safetensors runtime supporting `PaddleOCRVLForConditionalGeneration`.

### Shared image lineage

The GPU image in `docker/inference/Dockerfile.paddleocr` contains a shared base with the dependencies needed by both classic OCR and PaddleOCR-VL. Its `paddleocr` and `paddleocr-vl` final targets reuse those layers but bake different startup commands. The framework configurations use distinct immutable image references because runtime-framework persistence currently uses image identity as part of framework resolution.

```text
one Dockerfile and shared filesystem layers
  ├── target paddleocr    -> CMD serve.sh paddleocr
  │                         -> opencsghq/paddleocr:3.7.0
  └── target paddleocr-vl -> CMD serve.sh paddleocr-vl
                            -> opencsghq/paddleocr-vl:3.7.0
```

The two image repositories intentionally use the same `3.7.0` release version. They do not collide because their repository names differ, and their final OCI image configurations and manifest digests are distinct because their `CMD` values differ. Selecting the runtime framework therefore selects both the implementation image and its behavior; deployment code does not inject the framework name into the container. The existing classic `opencsghq/paddleocr:3.7.0` image must not be replaced when the new VL image is published.

### PaddleX owns the document pipeline

PaddleX remains responsible for:

- selecting the layout-analysis submodel;
- detecting layout regions and reading order;
- cropping document regions;
- invoking VL recognition;
- assembling per-page structured results and Markdown.

AIGateway only transforms the client request, selects the correct adapter, proxies to PaddleX, and normalizes the response.

## PaddleOCR-VL Version Mapping

PaddleX registers the PaddleOCR-VL series as independent top-level pipelines. Their APIs are similar, but their default layout and VL models differ.

| PaddleX pipeline | Layout-analysis model | VL-recognition model |
| --- | --- | --- |
| `PaddleOCR-VL` | `PP-DocLayoutV2` | `PaddleOCR-VL-0.9B` |
| `PaddleOCR-VL-1.5` | `PP-DocLayoutV3` | `PaddleOCR-VL-1.5-0.9B` |
| `PaddleOCR-VL-1.6` | `PP-DocLayoutV3` | `PaddleOCR-VL-1.6-0.9B` |

The runtime reads the canonical recognition name from `Global.model_name` in the downloaded repository's `inference.yml`. Repository paths and CSGHub-imported names whose source namespace is flattened into an underscore prefix are storage identities only and do not participate in pipeline selection.

| `inference.yml` model name | Selected pipeline |
| --- | --- |
| `PaddleOCR-VL-0.9B` | `PaddleOCR-VL` |
| `PaddleOCR-VL-<version>-0.9B` | `PaddleOCR-VL-<version>` |

For the currently supported repositories this produces:

| Metadata model name | Selected pipeline |
| --- | --- |
| `PaddleOCR-VL-0.9B` | `PaddleOCR-VL` |
| `PaddleOCR-VL-1.5-0.9B` | `PaddleOCR-VL-1.5` |
| `PaddleOCR-VL-1.6-0.9B` | `PaddleOCR-VL-1.6` |

Missing or invalid metadata and names outside the PaddleOCR-VL family fail startup. The discovered metadata value is written to `VLRecognition.model_name`, while the downloaded repository directory is written to `VLRecognition.model_dir`. PaddleX's pipeline-config lookup remains the final validation that the derived pipeline version exists in the pinned runtime.

The pipeline version, rather than custom CSGHub logic, selects `PP-DocLayoutV2` or `PP-DocLayoutV3`. `gen_pipeline.py` patches only `SubModules.VLRecognition`; it must not replace `SubModules.LayoutDetection`.

## System Architecture

```text
Client
  |
  | POST /v1/ocr (multipart file + model + options)
  v
AIGateway OCR handler
  |
  | resolve model and runtime_framework
  v
OCR adapter registry
  ├── paddleocr    -> classic PaddleX adapter -> /ocr
  └── paddleocr-vl -> PaddleX VL adapter      -> /layout-parsing
                                                   |
                                                   v
                                           PaddleX VL pipeline
                                           ├── LayoutDetection
                                           │   └── PP-DocLayoutV2/V3
                                           ├── VLRecognition
                                           │   └── selected local VL repo
                                           └── document assembly
```

The main implementation seams are:

| Responsibility | File |
| --- | --- |
| Runtime registration | `configs/inference/paddleocr-vl.json` |
| Runtime image | `docker/inference/Dockerfile.paddleocr` |
| Repository download | `docker/inference/paddleocr/entry.py` |
| Model metadata parsing | `docker/inference/paddleocr/model_metadata.py` |
| Framework and pipeline selection | `docker/inference/paddleocr/serve.sh` |
| Generated-config patching | `docker/inference/paddleocr/gen_pipeline.py` |
| Adapter registry and shared contract | `aigateway/component/adapter/ocr/adapter.go` |
| Classic PaddleX adapter | `aigateway/component/adapter/ocr/paddlex.go` |
| PaddleOCR-VL adapter | `aigateway/component/adapter/ocr/paddlex_vl.go` |
| Public request handling | `aigateway/handler/openai_ocr.go` |
| Normalized API types | `aigateway/types/ocr.go` |

## Deployment and Startup Flow

### Image-owned runtime mode

Selecting a runtime framework already selects its immutable `FrameImage`. The image target owns its PaddleOCR mode through the baked command:

```text
paddleocr image    -> /etc/csghub/serve.sh paddleocr
paddleocr-vl image -> /etc/csghub/serve.sh paddleocr-vl
```

`serve.sh` treats its first argument as authoritative. It accepts only `paddleocr` and `paddleocr-vl` and fails startup for a missing or unknown value. Repository names, deployment environment values, and user-provided engine arguments do not choose between classic OCR and VL parsing.

The CPU Dockerfile explicitly invokes the classic `paddleocr` mode. PaddleOCR-VL remains GPU-only.

### Selected repository download

`entry.py` downloads `REPO_ID` at `REVISION` from the deployment-provided CSGHub endpoint into:

```text
/workspace/<REPO_ID>
```

This is the user-selected primary model repository. For a bare PaddleOCR-VL repository, it contains the VL-recognition weights, not the complete PaddleX pipeline and all its submodels.

### Pipeline-selection precedence

`serve.sh` selects the pipeline in this order:

1. Use `PADDLEX_PIPELINE` when explicitly set.
2. Use `<model repo>/pipeline.yaml` when present.
3. For classic OCR, use `<model repo>/OCR.yaml` when present.
4. In `local-only` mode, fail if the repository is not a self-contained pipeline bundle.
5. For `paddleocr-vl` in hub mode, derive and generate the version-matched top-level pipeline configuration.
6. For a classic single recognition model, generate and patch the `OCR` configuration.
7. Otherwise start PaddleX's built-in `OCR` pipeline.

The runtime generates the VL configuration under the downloaded repository's runtime-owned directory:

```text
/workspace/<REPO_ID>/.csghub/<pipeline-name>.yaml
```

For example:

```bash
paddlex --get_pipeline_config PaddleOCR-VL-1.6 \
  --save_path /workspace/<REPO_ID>/.csghub
```

PaddleX deterministically writes `PaddleOCR-VL-1.6.yaml`. The runtime removes a stale generated copy first because `/workspace` can persist across restarts and the PaddleX CLI otherwise prompts before overwriting.

`gen_pipeline.py` points the selected recognition module at the downloaded repository and enables the optional document-preprocessor sub-pipeline so its models are available at request time:

```yaml
use_doc_preprocessor: true

SubModules:
  LayoutDetection:
    model_name: PP-DocLayoutV3
    model_dir: null

  VLRecognition:
    model_name: PaddleOCR-VL-1.6-0.9B
    model_dir: /workspace/<REPO_ID>

SubPipelines:
  DocPreprocessor:
    use_doc_orientation_classify: true
    use_doc_unwarping: true
```

This initializes `PP-LCNet_x1_0_doc_ori` and `UVDoc` when the service starts but does not require either operation to run for every request. Leaving their model directories and the layout `model_dir` unset is intentional: it activates PaddleX's established submodel-resolution behavior, which is also used by classic OCR detection. A local-only or explicitly supplied pipeline configuration owns its complete model layout and must initialize the same preprocessing capabilities if it accepts these request options.

## Model Source Strategy

### Hub mode

`PADDLEOCR_MODEL_SOURCE=hub` is the default. The runtime configures PaddleX's Hugging Face model source to use CSGHub's compatible API:

```text
PADDLE_PDX_HUGGING_FACE_ENDPOINT=<HF_ENDPOINT>/hf
PADDLE_PDX_MODEL_SOURCE=huggingface
HF_TOKEN=<ACCESS_TOKEN>
HF_HUB_OFFLINE=0
```

The selected VL repository is downloaded by `entry.py`. Required pipeline submodels, including `PP-DocLayoutV2` or `PP-DocLayoutV3`, are resolved by PaddleX from their `model_name` values and cached by PaddleX.

The runtime does not contain a separate `PP-DocLayoutV3` downloader and does not bake layout-model weights into the image. This keeps model versions independent from the runtime image and follows the same design as classic OCR submodel resolution.

### Local-only mode

`PADDLEOCR_MODEL_SOURCE=local-only` is the strict offline mode. The selected repository must include a self-contained `pipeline.yaml` and every required model directory.

Example:

```text
<repo>/
├── pipeline.yaml
└── models/
    ├── PP-DocLayoutV3/
    └── PaddleOCR-VL-1.6-0.9B/
```

The pipeline must point every required module at its local directory:

```yaml
SubModules:
  LayoutDetection:
    model_name: PP-DocLayoutV3
    model_dir: /workspace/<REPO_ID>/models/PP-DocLayoutV3
  VLRecognition:
    model_name: PaddleOCR-VL-1.6-0.9B
    model_dir: /workspace/<REPO_ID>/models/PaddleOCR-VL-1.6-0.9B
```

The runtime fails before generating a pipeline when a local-only repository does not contain `pipeline.yaml`. This prevents accidental network dependency in an air-gapped deployment.

## Public AIGateway Contract

### Request

```text
POST /v1/ocr
Content-Type: multipart/form-data
Authorization: Bearer <API key>
```

Required fields:

- `model`: deployed model ID;
- `file`: one image or PDF file.

Supported content types:

- `image/png`;
- `image/jpeg`;
- `image/webp`;
- `image/bmp`;
- `image/tiff`;
- `application/pdf`, only for `paddleocr-vl`.

The uploaded file limit is 20 MiB. The whole multipart request is bounded by an additional 1 MiB for form overhead.

Optional fields:

| Field | Classic OCR | PaddleOCR-VL |
| --- | --- | --- |
| `use_doc_orientation_classify` | forwarded | forwarded |
| `use_doc_unwarping` | forwarded | forwarded |
| `use_textline_orientation` | forwarded | rejected |
| `return_image` | maps to upstream `visualize` | maps to upstream `visualize` |
| `raw_response` | includes raw result | includes raw result |
| `page_ranges` | rejected when non-empty | rejected when non-empty |

For `paddleocr-vl`, `return_image` additionally requests upstream Markdown
images (`returnMarkdownImages=true`) and surfaces them in `pages[].images` as
base64. `raw_response` only controls whether the complete upstream response is
echoed in `raw_result`.

### PaddleX VL upstream request

AIGateway converts the multipart upload to PaddleX JSON:

```json
{
  "file": "<base64 file bytes>",
  "fileType": 1,
  "useDocOrientationClassify": false,
  "useDocUnwarping": false,
  "visualize": false,
  "returnMarkdownImages": false
}
```

`fileType` is `1` for images and `0` for PDFs. The adapter always sends both preprocessing booleans: omitted client options become `false`, while explicit values are preserved. This keeps preprocessing off for ordinary requests even though the models are initialized. `returnMarkdownImages` is enabled when `return_image=true`, so the upstream `markdown.images` mapping is populated and the normalized response can carry it in `pages[].images`.

Compatibility note: before this change, `raw_response=true` implicitly requested
`returnMarkdownImages=true`, so `raw_result` carried `markdown.images`. Clients
that relied on that must now pass `return_image=true` explicitly; `raw_response`
only controls whether the complete upstream response is echoed in `raw_result`.

The adapter proxies this request to `/layout-parsing`. A model-level endpoint path may override the adapter's default endpoint.

### Normalized response

```json
{
  "id": "ocr_<id>",
  "object": "ocr.result",
  "created": 1760000000,
  "model": "owner/model:deploy-id",
  "text": "combined plain text",
  "pages": [
    {
      "index": 0,
      "text": "page plain text",
      "markdown": "page markdown",
      "lines": [],
      "images": [
        {
          "name": "imgs/img_in_image_box_755_185_1522_685.jpg",
          "content": "<base64 image bytes>"
        }
      ]
    }
  ],
  "usage": {
    "pages": 1,
    "images": 1
  }
}
```

Response mapping:

| PaddleX field | AIGateway field |
| --- | --- |
| `layoutParsingResults[i].markdown.text` | `pages[i].markdown` |
| `layoutParsingResults[i].prunedResult.parsing_res_list[].block_content` | joined into `pages[i].text` |
| `layoutParsingResults[i].markdown.images` | `pages[i].images`, only with `return_image=true` |
| all page text | joined into top-level `text` |
| number of layout parsing results | `usage.pages` |
| complete PaddleX response | `raw_result`, only with `raw_response=true` |

The upstream `markdown` field is an object, while `OCRPage.Markdown` is deliberately a string. Only `markdown.text` is placed in the normalized page. The `markdown.images` mapping (filename -> base64) is normalized into `pages[i].images` with a stable filename order when `return_image=true`; it is omitted otherwise.

`outputImages` and `markdown.images` must not be mapped to `pages[].image_url`: PaddleX may return base64 data rather than stable URLs, and AIGateway does not upload these payloads to object storage in this version. `pages[].images` carries the raw base64 content so clients can resolve the Markdown references themselves.

The normalized Markdown is not guaranteed to be self-contained when PaddleX emits relative image references. Clients that need these assets request `return_image=true` and resolve the Markdown references against `pages[].images`; the complete `markdown.images` mapping also remains available in `raw_result` with `raw_response=true`.

VL results use Markdown and page text rather than classic `OCRLine` values. Tables, formulas, and layout blocks remain in Markdown or `raw_result` instead of being represented as ordinary OCR lines.

## Adapter and Proxy Behavior

The OCR adapter registry evaluates the VL adapter before the classic adapter. This matters because the classic adapter also supports broader legacy task/provider matching.

```text
runtime_framework=paddleocr-vl
  -> PaddleXVLAdapter
  -> /layout-parsing

runtime_framework=paddleocr
  -> PaddleXAdapter
  -> /ocr
```

The response wrapper buffers the non-streaming PaddleX response, passes upstream HTTP errors through unchanged, and normalizes successful responses with the selected adapter. Invalid successful payloads become an `upstream_response_error` response.

The VL adapter fails normalization when:

- PaddleX reports a non-zero `errorCode`;
- `layoutParsingResults` is empty;
- a page has no `markdown.text`;
- a page has no `prunedResult.parsing_res_list`.

## Usage, Authentication, and Observability

The handler follows the standard AIGateway inference flow:

1. authenticate the request;
2. resolve the deployed model target;
3. select the OCR adapter;
4. validate the input against adapter capabilities;
5. run balance checks when applicable;
6. proxy and normalize the PaddleX response;
7. record OCR usage asynchronously.

OCR usage is derived from the normalized response and records page and image counts. Modal-generation traces include the selected model, status, page count, image count, `page_ranges`, and `return_image` metadata.

Runtime model downloads use the deployment access token. AIGateway applies the resolved model's configured upstream authentication headers before proxying.

## Failure Behavior

| Condition | Behavior |
| --- | --- |
| Missing or unknown image runtime-mode argument | Container startup fails |
| Unknown PaddleOCR model source | Container startup fails |
| Missing or unsupported `inference.yml` model identity | Container startup fails |
| Missing offline pipeline bundle | Container startup fails |
| PaddleX fails to generate the selected pipeline config | Container startup fails |
| PDF sent to classic OCR | AIGateway returns a client error |
| `use_textline_orientation` sent to VL | AIGateway returns a client error |
| Non-empty `page_ranges` | AIGateway returns a client error |
| Upstream HTTP error | Status and body pass through |
| Invalid successful upstream response | AIGateway returns `upstream_response_error` |

## Verification Requirements

### Runtime

- Verify all three supported PaddleOCR-VL pipeline names generate the expected YAML filename.
- Verify each `inference.yml` model identity selects its matching pipeline.
- Verify the base pipeline retains `PP-DocLayoutV2`.
- Verify the 1.5 and 1.6 pipelines retain `PP-DocLayoutV3`.
- Verify generated patching preserves layout-model selection, points only `VLRecognition` at the deployed repository, and enables `DocPreprocessor`.
- Verify layout and preprocessing submodels download through the configured model source in hub mode.
- Verify `PP-LCNet_x1_0_doc_ori` and `UVDoc` initialize before serving becomes ready.
- Verify a complete local-only bundle starts without network access.
- Verify missing local-only submodels fail clearly.
- Verify missing or unknown image runtime modes and non-PaddleOCR-VL model names fail startup.
- Require an explicit mapping and compatibility validation before declaring a future PaddleOCR-VL version supported.

### AIGateway

- Classic image OCR regression through `/ocr`.
- VL image parsing through `/layout-parsing`.
- Multi-page PDF parsing and correct page count.
- Reading-order, table, and formula fixtures.
- Image/PDF `fileType` mapping.
- Omitted preprocessing options serialize upstream as explicit `false` values.
- Orientation and unwarping opt-ins independently preserve explicit `true` values.
- Unsupported classic PDF and VL text-line-orientation errors.
- Rejection of non-empty `page_ranges`.
- `markdown.text` string normalization.
- Populated, missing, and null `markdown.images` become `pages[].images` only
  with `return_image=true`, in stable filename order.
- Complete raw-response gating.
- `return_image=true` requests upstream `returnMarkdownImages` and surfaces
  base64 images in the normalized response.
- Upstream HTTP-error passthrough and invalid-success-response handling.
- OCR usage and trace metadata.

### Operational acceptance

Before publishing a new runtime tag, record:

- cold-start and submodel-download time;
- GPU memory use;
- image and multi-page PDF latency;
- reading-order, table, formula, and Markdown quality;
- behavior across pod restarts with a warm model cache;
- failure diagnostics when a required submodel cannot be downloaded.

Do not build or publish an image as part of documentation-only changes.

## Follow-ups

- Add a separately designed external `vllm-server` or `sglang-server` recognition topology if performance requires it. This needs lifecycle, readiness, networking, credentials, and resource-accounting design.
- Add typed document blocks only through a new normalized API contract; do not overload `OCRLine`.
- Add Markdown asset hosting only with an explicit storage, lifetime, authorization, and URL contract.
- Revisit PDF size and timeout limits using production document distributions.
- Replace image-based runtime-framework identity with a stable framework ID/name in a broader runtime architecture change.

## References

- PaddleX PaddleOCR-VL pipeline: https://paddlepaddle.github.io/PaddleX/3.7/en/pipeline_usage/tutorials/ocr_pipelines/PaddleOCR-VL.html
- PaddleX layout analysis: https://paddlepaddle.github.io/PaddleX/latest/en/module_usage/tutorials/ocr_modules/layout_analysis.html
- vLLM PaddleOCR-VL recipe: https://docs.vllm.ai/projects/recipes/en/stable/PaddlePaddle/PaddleOCR-VL.html
- GitLab issue: https://git-devops.opencsg.com/product/starhub/starhub-server/-/issues/1434
