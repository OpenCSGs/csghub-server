*[English](README_en.md) ∙ [简体中文](README_cn.md)*

`CSGHub Server` is a part of the open source and reliable large model assets management platform - [CSGHub](https://github.com/OpenCSGs/CSGHub/). It focuses on management of models、datasets and other LLM assets through REST API。

## Key Features：
- Creation and Management of users and orgnizations
- Auto-tagging of model and dataset labels
- Search for users, organizations, models, and data
- Online preview of dataset files, like `.parquet` file
- Content moderation for both text and image 
- Download of individual files, including LFS files
- Tracking of model and dataset activity data, such as downloads and likes volume

## Demo
In order to help users to quickly understand the features and usage of CSGHub, we have recorded a demo video. You can watch this video to get a quick understanding of the main features and operation procedures of this program.
- CSGHub Demo video is as blew，you can also check it at [YouTube](https://www.youtube.com/watch?v=SFDISpqowXs) or [Bilibili](https://www.bilibili.com/video/BV12T4y187bv/)
<video width="658" height="432" src="https://github-production-user-asset-6210df.s3.amazonaws.com/3232817/296556812-205d07f2-de9d-4a7f-b3f5-83514a71453e.mp4"></video>

Please visit the [OpenCSG website](https://portal.opencsg.com/models) to experience the powerful management features. 

## Quick Start
> System resource requirements: 4c CPU/8GB memory

Please install Docker yourself. This project has been tested in Ubuntu22 environment.

You can quickly deploy the localized `CSGHub Server` service through docker-compose:
```shell
# The API token should be at least 128 characters long, and HTTP requests to csghub-server require the API token to be sent as a Bearer token for authentication.
export STARHUB_SERVER_API_TOKEN=<API token>
mkdir -m 777 gitea minio_data
curl -L https://raw.githubusercontent.com/OpenCSGs/csghub-server/main/docker-compose.yml -o docker-compose.yml
docker-compose -f docker-compose.yml up -d
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
- [x] Support more Git Servers: Currently supports Gitea, and plans to support mainstream Git repositories in the future.
- [x] Git LFS: Git LFS supports large files, and supports Git command operations and online download through the Web UI. 
- [x] DataSet online viewer: Data set preview, supports the Top20/TopN loading preview of LFS format data sets. 
- [x] Model/Dataset AutoTag: Supports custom metadata and automatic extraction of model/dataset tags. 
- [x] S3 Protocol Support: Supports S3 (MinIO) storage protocol, providing higher reliability and storage cost-effectiveness.
- [ ] Model format convert: Conversion of mainstream model formats.
- [x] Model oneclick deploy: Supports integration with OpenCSG llm-inference, one-click to start model inference.

## License
We use the Apache 2.0 license, the content of which is detailed in the `LICENSE` file.

## Contributing
If you wish to contribute, please follow the [Contribution Guidelines](docs/en/contributing.md). We are very excited about your contributions!

## Acknowledgments
This project is based on open source projects such as Gin, DuckDB, minio, and Gitea. We would like to express our sincere gratitude to them for their open source contributions!

### CONTACT WITH US
If you meet any problem during usage, you can contact with us by any following way:
1. initiate an issue in github
2. join our WeChat group by scaning wechat helper qrcode
3. join our offical discord channel: [OpenCSG Discord Channel](https://discord.gg/bXnu4C9BkR)
4. join our slack workspace:[OpenCSG Slack Channel](https://join.slack.com/t/opencsghq/shared_invite/zt-2fmtem7hs-s_RmMeoOIoF1qzslql2q~A)
<div style="display:inline-block">
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/wechat-assistant-new.png" width='200'>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/discord-qrcode.png" width='200'>
  &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/slack-qrcode.png" width='200'>
</div>
