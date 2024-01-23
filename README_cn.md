*[English](README_en.md) ∙ [简体中文](README_cn.md)*

`CSGHub Server` is a part of the open source and reliable large model assets management platform - `CSGHub`. It focus on management of models and datasets through REST API。

## Key Features：
- Creation and Management of users and orgnizations
- Auto-tagging of model and dataset labels
- Search for users, organizations, models, and data
- Online preview of dataset files, like `.parquet` file
- Content moderation for both text and image 
- Download of individual files, including LFS files
- Tracking of model and dataset activity data, such as downloads and likes volume

## Demo
To help you better understand the features and usage of `CSGHub Server`, we have recorded a series of demonstration videos. These videos will quickly introduce you to the main features and operating steps of the project.
- Web GUI features demo: Demo Video
- Git operations demo: Demo Video

Please visit the [OpenCSG website](https://portal.opencsg.com/) to experience the powerful management features. The "Developer" module is for LLM model and dataset.

## Quick Start
> System resource requirements: 4c CPU/8GB memory

Please install Docker yourself. This project has been tested in Ubuntu22 environment.

You can quickly deploy the localized `CSGHub Server` service through docker-compose:
```
docker-compose up -d -f https://github.com/opencsginc/starhub/blob/main/docker/docker-compose.yaml
```

## Technical Architecture
<div align=center>
  <img src="docs/csghub_server-arch.png" alt="csghub-server architecture" width="800px">
</div>

### Extensible and customizable

- Supports different git servers, such as Gitea, GitLab, etc.
- Supports flexible configuration of the LFS storage system, and you can choose to use local or any third-party cloud storage service that is compatible with the S3 protocol.
- Enable content moderation on demand, and choose any third-party content moderation service.

## Roadmap
- [ ] Support more Git Servers: Currently supports Gitea, and plans to support mainstream Git repositories in the future.
- [x] Git LFS: Git LFS supports large files, and supports Git command operations and online download through the Web UI. 
- [x] DataSet online viewer: Data set preview, supports the Top20/TopN loading preview of LFS format data sets. 
- [x] Model/Dataset AutoTag: Supports custom metadata and automatic extraction of model/dataset tags. 
- [x] S3 Protocol Support: Supports S3 (MinIO) storage protocol, providing higher reliability and storage cost-effectiveness.
- [ ] Model format convert: Conversion of mainstream model formats.
- [ ] Model oneclick deploy: Supports integration with OpenCSG llm-inference, one-click to start model inference.

## License
We use the Apache 2.0 license, the content of which is detailed in the `LICENSE` file.

## Contributing
If you'd like to contribute, start by cloning the project. Then follow the [configuration documentation](docs/en/config.md) to set up the project locally, and refer to the [database migration documentation](docs/en/migration.md) for adding new features. We highly appreciate your contributions!

## Acknowledgments
This project is based on open source projects such as Gin, DuckDB, minio, and Gitea. We would like to express our sincere gratitude to them for their open source contributions!

## Contact us
If you have any questions during use, you can raise an issue on GitHub.
