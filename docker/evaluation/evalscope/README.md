# Civil Comments 自定义数据集支持

## 📋 快速开始

支持从环境变量 `DATASET_IDS` 动态注册任意组织的 civil_comments 数据集。

### 使用示例

```bash
export DATASET_IDS="James/civil_comments"
export MODEL_IDS="your-model"
export USE_CUSTOM_DATASETS="false"
export EVALSCOPE_GENERATION_CONFIG='{"max_tokens":30000,"do_sample":true,"temperature":0.6,"top_p":0.95,"top_k":20}'
export EVALUATION_LIMIT=10
./start.sh
```

### 预期日志

```
[DEBUG] Successfully imported EvalScope v1.10.0 API
✓ Custom dataset 'James/civil_comments' registered successfully as 'James_civil_comments'
[SUCCESS] Found task name: James_civil_comments
Loading civil_comments from remote: James/civil_comments, split: test
```

## 🎯 核心特性

- ✅ **动态注册** - 从 DATASET_IDS 自动识别
- ✅ **多组织支持** - google、James、任意组织
- ✅ **零配置** - 只需设置环境变量
- ✅ **详细日志** - 完整的调试信息

## 📚 文档

| 文档 | 说明 |
|------|------|
| `README.md` | 本文档 - 快速开始 |
| `CUSTOM_DATASETS.md` | 详细使用说明 |
| `REGISTRY_FIX.md` | Evalscope v1.1.1 注册方法说明 |
| `WRAPPER_SOLUTION.md` | 包装脚本解决方案说明 |

## 🔧 技术细节

### EvalScope v1.10.0 API

**正确的导入：**
```python
from evalscope.api.benchmark import DefaultDataAdapter
from evalscope.api.benchmark.meta import BenchmarkMeta
from evalscope.api.registry import BENCHMARK_REGISTRY, register_benchmark
```

**注册方法：**
```python
# 创建 BenchmarkMeta
meta = BenchmarkMeta(
    name='benchmark_name',
    dataset_id='org/dataset',
    data_adapter=DefaultDataAdapter,
    eval_split='test',
)

# 直接注册到 BENCHMARK_REGISTRY
BENCHMARK_REGISTRY['benchmark_name'] = meta
```

## 📝 支持的数据集

### AIME2025

- **仓库**: `evalscope/aime25`
- **子集**: 由 EvalScope 官方 benchmark metadata 管理
- **适配器**: EvalScope 1.10.0 官方 `AIME25Adapter`

### civil_comments (任意组织)

- **格式**: `{organization}/civil_comments`
- **示例**: 
  - `google/civil_comments`
  - `James/civil_comments`
  - `your-org/civil_comments`
- **Split**: test（默认）
- **任务**: 毒性检测（二分类）

## 🐛 故障排除

### 问题 1: ImportError

**症状：** 各种导入错误

**解决方案：** 使用正确的导入路径：
```python
from evalscope.api.benchmark import DefaultDataAdapter
from evalscope.api.benchmark.meta import BenchmarkMeta
from evalscope.api.registry import BENCHMARK_REGISTRY
```

### 问题 2: dataset_tasks is empty

**检查：**
1. 查看注册日志是否显示成功
2. 检查 DATASET_IDS 是否正确传递
3. 确认数据集名称包含 "civil_comments"

参考 `REGISTRY_FIX.md` 获取详细信息。

## ✨ 文件结构

```
evalscope/
├── custom_datasets.py      # 核心实现
├── evalscope_wrapper.py    # Evalscope 包装脚本（关键！）
├── register_custom.py       # 注册脚本
├── get_task.py             # 任务查找（已修改）
├── start.sh                # 启动脚本（已修改）
├── README.md               # 本文档
├── CUSTOM_DATASETS.md      # 详细说明
├── REGISTRY_FIX.md         # 注册方法说明
└── WRAPPER_SOLUTION.md     # 包装脚本解决方案
```

**关键组件：**
- **`evalscope_wrapper.py`**: 在同一进程中先注册数据集再运行 evalscope
- **`start.sh`**: 使用 `python evalscope_wrapper.py` 代替 `evalscope` 命令

## 🚀 扩展

添加新的自定义数据集，编辑 `custom_datasets.py`：

```python
# 在 register_custom_datasets() 函数中
for dataset_id in dataset_id_list:
    if 'your_dataset' in dataset_id.lower():
        # 实现注册逻辑
```

---

**版本**: EvalScope v1.10.0
**状态**: ✅ 已测试并修复

