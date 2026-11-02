---
library_name: sglang-diffusion
tags:
- sglang-diffusion
- overlay
- minimax-h3
- fasth3
---

# FastH3 8-Step V2 Overlay

Overlay metadata repo for using `FastVideo/FastVideo-FastH3-8-Step-V2`
with native SGLang Diffusion MiniMax-H3 support.

This repo contains only overlay metadata and materialization logic. Source weights
remain in `FastVideo/FastVideo-FastH3-8-Step-V2`; SGLang's
registry pins a revision of this repo so re-materializing the model does not
drift.

The materializer reshapes the flat native-Diffusers release into the base
MiniMax-H3 partition layout SGLang's loaders consume:

- `model_index.json` declares the t2va-only release contract
  (`tasks: ["t2va"]`, video/audio sigma shifts 10/3, and the trained
  eight DMD rungs `[999, 874, 749, 624, 500, 375, 250, 125]`).
- `_overlay/materialize.py` re-serializes the video VAE into the fused source
  form (bit-identical tensor values; names and fused-QKV row order differ),
  derives the `rope.inv_freq` buffer the Diffusers export drops, and patches
  the VAE config class names. Everything else is symlinked from the source
  snapshot.

Usage (no manual steps; SGLang resolves this overlay automatically):

```bash
sglang serve \
  --model-path FastVideo/FastVideo-FastH3-8-Step-V2 \
  --num-gpus 4 \
  --component-attention-backends transformer=video_sparse_attn_h3
```

The model inherits the MiniMax-H3 Community License from its base checkpoint.
