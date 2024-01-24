# 贡献指南

欢迎您对 CSGHub Server 项目的贡献！我们非常感谢您对这个开源项目的兴趣。

CSGHub Server是开源大模型资产管理平台 CSGHub 的服务端部分的开源项目，提供基于REST API的模型和数据集等大模型资产管理功能。

## 贡献流程

为了对项目做出贡献，请按照以下的 "fork and pull request" 工作流程进行操作。请注意，除非您是维护者，否则请不要直接推送更改到该存储库。

1. 在 GitHub 上 fork 本项目的存储库(https://github.com/OpenCSGs/csghub-server) 到您的账号下。
2. 进入您 fork 后的存储库，进行修改和改进。
3. 在您的 fork 后的存储库中，创建一个新的分支来承载您的修改。建议基于 main 分支创建新分支。
4. 在新分支上进行修改和改进。
5. 当您完成修改后，向原存储库的 main 分支提交 Pull Request（简称 PR）。
6. 维护者将会审查您的 PR，并提供反馈和讨论。
7. 在经过必要的修改和讨论后，您的 PR 将被合并到 main 分支中。

请确保您的贡献符合以下要求：

- 代码风格与项目保持一致。
- 新功能或改进应该经过适当的测试。
- 文档的新增或修改应该清晰明了，以便其他开发者能够理解和使用。

## 报告问题和提出建议

如果您发现了问题、改进或功能请求，请在我们的 [Issues 页面](https://github.com/OpenCSGs/csghub-server/issues)进行报告。我们会定期查看并回复您的问题。

在报告问题和提出建议时，请遵循以下准则：

- 尽可能提供详细的信息，说明具体出错的地方、错误类型以及相关的代码片段等。"某个功能不工作" 这样的描述对于解决问题并不具有帮助性。请提供您运行的代码和相关的上下文信息，以便我们能够重现并定位问题。 
- 如果需要包含大段的代码、日志或追踪信息，你可以使用 `<details>` 和 `</details>` 标签将它们包裹起来。这样可以折叠内容，使问题更易于阅读和跟踪。你可以参考[这个链接](https://developer.mozilla.org/en/docs/Web/HTML/Element/details)了解更多关于折叠内容和使用HTML中的 `<details>` 标签的信息。

## 标签说明

我们使用标签对问题和 Pull Request 进行分类和归类，详细的标签说明请参考[这个页面](https://github.com/OpenCSGs/csghub-server/labels)。

## 本地开发

你可以使用 docker-compose 来拉起 CSGHub Server 项目，拉起后会单独启动 CSGHub Server 项目，这是一个后端项目，作用是为 [CSGHub](https://github.com/OpenCSGs/CSGHub) 提供后端 API 支持。你也可以根据 [配置文档](config.md) 和 [数据库迁移文档](migration.md) 来配置并在本地编译启动。

感谢您对 CSGHub Server 项目的贡献！我们期待您的参与和建议。
