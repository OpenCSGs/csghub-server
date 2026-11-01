*[English](README.md) ∙ [简体中文](README_zh-hans.md)*

`CSGHub Server`是开源、可信的大模型资产管理平台`CSGHub`的服务端部分开源项目，提供基于REST API的模型和数据集管理功能。

## 主要功能：
- 用户和组织的创建和管理 
- 模型、数据集托管，支持以https或git协议的方式上传和下载模型、数据集文件
- 模型、数据集标签的自动生成
- 用户、组织、模型和数据的搜索
- 数据集文件在线预览，目前支持`.parquet`格式文件
- 文本、图像内容审核
- 单个文件下载，包括LFS文件下载
- 模型、数据集活跃度数据跟踪，如下载量、Like量等

## 功能演示
为了帮助您更直观地了解`CSGHub` 的功能和使用方法，我们录制了一系列演示视频。您可以通过观看这些视频，快速了解本项目的主要特性和操作流程。
- CSGHub Portal 功能演示：演示视频
- CSGHub Git 操作演示：演示视频

更完整的功能请移步[OpenCSG官网](https://portal.opencsg.com/)，体验“开发者”模块的强大管理功能。

## 快速使用
系统资源需求: 4c CPU/8GB内存
请准备自行安装docker程序，本项目已在 Ubuntu22 环境下中完成测试。

您可以通过docker-compose快速部署本地化的csghub-server服务：
```
docker-compose up -d -f https://github.com/opencsginc/starhub/blob/main/docker/docker-compose.yaml
```

## 技术架构
<div align=center>
  <img src="docs/csghub_server-arch.png" alt="csghub-server architecture" width="800px">
</div>

### 可扩展可定制
- 支持不同的Git Server，如gitea，gitlab等
- 支持灵活配置LFS存储系统，可选择使用本地或第三方兼容S3协议的任意云存储服务
- 按需开启内容审核，选择任意第三方内容审核服务

## 技术规划
- 支持更多Git Server: 目前内置了对gitea的支持，未来计划实现对主流Git仓库的支持
- 支持Git LFS: Git LFS支持超大文件， 支持git命令操作和Web UI在线下载
- 数据集在线预览: 数据集预览，支持LFS格式数据集的Top20/TopN加载预览
- 模型和数据集自动打标签:：支持自定义元数据和自动化提取模型/数据集标签
- S3协议兼容: 支持S3(MinIO)存储协议，更高的可靠性和存储性价比
- 模型格式转换: 主流模型格式转化
- 模型一键部署: 支持与OpenCSG llm-inference集成， 一键启动模型推理

## License
我们使用Apache 2.0协议，协议内容详见`LICENSE`文件。

## 参与贡献
如果你想参与贡献，可以先克隆项目，然后根据 [配置文档](docs/zh-CN/config.md) 来在本地启动项目，根据 [数据库迁移文档](docs/zh-CN/migration.md) 来添加功能。我们非常期待你的贡献！

## 致谢
本项目基于Gin, DuckDB, minio, gitea等开源项目，在此深深感谢他们的开源贡献！

## 联系我们
使用过程中的任何问题， 您可以在github 发起issue或者加入我们的微信讨论群.
<div align=center>
  <img src="docs/wechat_group.jpg" alt="wechat group" width="500px">
</div>