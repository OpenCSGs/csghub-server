# Custom Datasets for Evalscope

## 支持的自定义数据集

### civil_comments (任意组织)

Civil Comments 数据集用于毒性检测评估。**支持来自任意组织的 civil_comments 数据集**。

**支持的数据集 ID 格式:**
- `google/civil_comments` - Google 官方版本
- `abc/civil_comments` - ABC 组织的 fork 版本
- `your-org/civil_comments` - 任意组织的版本
- 任何包含 "civil_comments" 的数据集 ID

**数据集划分 (splits):**
- `train` - 训练集（不用于评估）
- `validation` - 验证集（用于超参数调优）
- `test` - 测试集（**用于最终评估，默认使用**）

**默认行为:**
系统默认使用 `test` 集进行评估，这是标准的评估实践。test 集包含模型从未见过的数据，能够真实反映模型的泛化能力。

**动态注册机制:**
系统会自动从环境变量 `DATASET_IDS` 中读取数据集 ID，识别包含 "civil_comments" 的数据集，并动态注册使用实际的 dataset_id。

**使用方式:**

在运行评估时，将 `DATASET_IDS` 设置为包含 civil_comments 的数据集 ID，系统会自动识别并注册。

**示例:**

```bash
# 使用 Google 官方版本
DATASET_IDS="google/civil_comments" \
MODEL_IDS="your-model-id" \
./start.sh

# 使用 ABC 组织的版本
DATASET_IDS="abc/civil_comments" \
MODEL_IDS="your-model-id" \
./start.sh

# 同时评估多个数据集（只有 civil_comments 会被注册）
DATASET_IDS="abc/civil_comments,test/hellaswag" \
MODEL_IDS="your-model-id" \
./start.sh
```

## 高级配置

### 使用不同的数据集划分

如果需要使用 `validation` 集而非 `test` 集进行评估，可以修改 `custom_datasets.py` 中的注册代码：

```python
# 在 register_custom_datasets() 函数中修改
civil_comments_meta = BenchmarkMeta(
    name='civil_comments',
    dataset_id='google/civil_comments',
    dataset_name='civil_comments',
    subset_list=['default'],
    metrics=['accuracy', 'f1'],
    few_shot_num=0,
    benchmark_cls=CivilComments,
    split='validation'  # 修改这里：使用 validation 集
)
```

**注意**: 不推荐使用 `train` 集进行评估，因为会导致评估结果失真。

## 添加新的自定义数据集

要添加新的自定义数据集支持，请编辑 `custom_datasets.py` 文件：

1. 创建一个继承自 `Benchmark` 的类
2. 实现 `load()` 和 `format_sample()` 方法
3. 在 `register_custom_datasets()` 函数中注册新数据集

## 技术说明

- 自定义数据集在评估开始前自动注册
- 不修改 evalscope 源码，仅通过插件方式扩展
- 支持本地路径和远程数据集加载

