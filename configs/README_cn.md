### README_cn.md

```markdown
# 说明文档

## 适配的算力类型

目前可适配的算力有：

- Ascend (NPU)
- NVIDIA (GPU)
- Enflame (Enflame)
- Cambricon (MLU)

computing_power_type:
- gpu
- npu
- enflame
- mlu

## 支持的引擎

支持的引擎有：

- 推理
- 微调
- 评估

## 自定义推理引擎配置

您可以使用以下 JSON 配置自定义推理引擎：

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
  "supported_archs": [],
  "supported_models": []
}
```

## 镜像构建说明

Docker 镜像需要根据 docker/inference 目录中提供的 Dockerfile 的规范进行构建。构建完成后，请将镜像上传到您选择的镜像仓库。JSON 配置中的 image 字段应包含镜像的完整路径。