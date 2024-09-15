*[English](README_en.md) ∙ [简体中文](README_cn.md) ∙ [日本語](README_ja.md)*

`CSGHub Server`は、オープンソースで信頼性の高い大規模モデル資産管理プラットフォーム - [CSGHub](https://github.com/OpenCSGs/CSGHub/)の一部です。REST APIを通じてモデル、データセット、その他のLLM資産の管理に焦点を当てています。

## 主な機能：
- ユーザーと組織の作成と管理
- モデルとデータセットのラベルの自動タグ付け
- ユーザー、組織、モデル、データの検索
- データセットファイルのオンラインプレビュー、例えば `.parquet` ファイル
- テキストと画像のコンテンツモデレーション
- 個々のファイルのダウンロード、LFSファイルを含む
- モデルとデータセットのアクティビティデータの追跡、ダウンロード数やいいね数など

## デモ
CSGHubの機能と使用方法を迅速に理解するために、デモビデオを録画しました。このビデオを視聴することで、プログラムの主な機能と操作手順を迅速に理解できます。
- CSGHubのデモビデオは以下の通りです。また、[YouTube](https://www.youtube.com/watch?v=SFDISpqowXs)や[Bilibili](https://www.bilibili.com/video/BV12T4y187bv/)でもご覧いただけます。
<video width="658" height="432" src="https://github-production-user-asset-6210df.s3.amazonaws.com/3232817/296556812-205d07f2-de9d-4a7f-b3f5-83514a71453e.mp4"></video>

強力な管理機能を体験するには、[OpenCSGウェブサイト](https://portal.opencsg.com/models)をご覧ください。

## クイックスタート
> システムリソース要件: 4c CPU/8GBメモリ

Dockerをインストールしてください。このプロジェクトはUbuntu22環境でテストされています。

docker-composeを使用してローカライズされた`CSGHub Server`サービスを迅速にデプロイできます：
```shell
# APIトークンは少なくとも128文字の長さである必要があり、csghub-serverへのHTTPリクエストにはAPIトークンをBearerトークンとして送信して認証を行う必要があります。
export STARHUB_SERVER_API_TOKEN=<API token>
mkdir -m 777 gitea minio_data
curl -L https://raw.githubusercontent.com/OpenCSGs/csghub-server/main/docker-compose.yml -o docker-compose.yml
docker-compose -f docker-compose.yml up -d
```

## 技術アーキテクチャ
<div align=center>
  <img src="docs/csghub_server-arch.png" alt="csghub-server architecture" width="800px">
</div>

### 拡張性とカスタマイズ性
- Gitea、GitLabなどの異なるGitサーバーをサポート
- LFSストレージシステムの柔軟な構成をサポートし、S3プロトコルに対応したローカルまたは任意のサードパーティクラウドストレージサービスを使用できます
- 必要に応じてコンテンツモデレーションを有効にし、任意のサードパーティコンテンツモデレーションサービスを選択できます

## ロードマップ
- [x] さらに多くのGitサーバーをサポート: 現在はGiteaをサポートしており、将来的には主流のGitリポジトリをサポートする予定です。
- [x] Git LFS: Git LFSは大きなファイルをサポートし、Gitコマンド操作とWeb UIを通じたオンラインダウンロードをサポートします。
- [x] データセットのオンラインビューア: データセットのプレビュー、LFS形式のデータセットのTop20/TopNの読み込みプレビューをサポートします。
- [x] モデル/データセットの自動タグ付け: カスタムメタデータとモデル/データセットタグの自動抽出をサポートします。
- [x] S3プロトコルのサポート: S3（MinIO）ストレージプロトコルをサポートし、より高い信頼性とストレージコスト効率を提供します。
- [ ] モデルフォーマットの変換: 主流のモデルフォーマットの変換。
- [x] モデルのワンクリックデプロイ: OpenCSG llm-inferenceとの統合をサポートし、ワンクリックでモデル推論を開始します。

## ライセンス
Apache 2.0ライセンスを使用しています。詳細は`LICENSE`ファイルをご覧ください。

## 貢献
貢献したい場合は、[貢献ガイドライン](docs/en/contributing.md)に従ってください。貢献を非常に楽しみにしています！

## 謝辞
このプロジェクトは、Gin、DuckDB、minio、Giteaなどのオープンソースプロジェクトに基づいています。これらのオープンソースの貢献に心から感謝します！

### お問い合わせ
使用中に問題が発生した場合は、以下のいずれかの方法でお問い合わせください：
1. GitHubでissueを発行する
2. WeChatヘルパーのQRコードをスキャンしてWeChatグループに参加する
3. 公式Discordチャンネルに参加する: [OpenCSG Discord Channel](https://discord.gg/bXnu4C9BkR)
4. Slackワークスペースに参加する: [OpenCSG Slack Channel](https://join.slack.com/t/opencsghq/shared_invite/zt-2fmtem7hs-s_RmMeoOIoF1qzslql2q~A)
<div style="display:inline-block">
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/wechat-assistant-new.png" width='200'>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/discord-qrcode.png" width='200'>
  &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
<img src="https://github.com/OpenCSGs/csghub/blob/main/docs/images/slack-qrcode.png" width='200'>
</div>
