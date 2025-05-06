# README

## Supported Computing Power Types

Currently, the following computing power types are supported:

- Ascend (NPU)
- NVIDIA (GPU)
- Enflame (Enflame)
- Cambricon (MLU)

computing_power_type:
- gpu
- npu
- enflame
- mlu

## Supported Engines

The following engines are supported:

- Inference
- Fine-tuning
- Evaluation

## Custom Inference Engine Configuration

You can customize the inference engine by using the following JSON configuration:

```json
{
  "engine_name": "tgi",
  "engine_version": "3.1",
  "container_port": "8000",
  "model_format": "safetensors",
  "engine_images": [
    {
      "computing_power_type": "gpu",
      "image": "tgi:3.1"
    }
  ],
  "supported_archs": [""],
  "supported_models": [""]
}
```
## Image Building Instructions

The Docker image needs to be built according to the specifications in the provided Dockerfile located in the `docker/inference`. After building the image, please upload it to your chosen image repository. The image field in the JSON configuration should contain the full path to the image.