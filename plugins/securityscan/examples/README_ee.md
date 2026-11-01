# 本地 ModelScan 插件样例（EE）

[`modelscan_sample_ee.go`](modelscan_sample_ee.go) 使用现有 `builder/plugins_manager` 的 HashiCorp go-plugin/gRPC 机制，在独立进程中调用 ModelScan CLI。它从 Moderation 的限时 HTTPS 文件地址下载 `.pkl` 模型文件，返回发现项和逐文件覆盖结果。样例不执行或反序列化待扫描模型。

## 构建与预安装

```sh
python3 -m venv /tmp/modelscan-plugin-venv
/tmp/modelscan-plugin-venv/bin/pip install 'modelscan==0.8.6'
mkdir -p /starhub-bin/plugins/modelscan/sample-1
go build -tags=ee -o /starhub-bin/plugins/modelscan/sample-1/modelscan ./plugins/securityscan/examples
sha256sum /starhub-bin/plugins/modelscan/sample-1/modelscan
```

`/starhub-bin/plugins` 是示例路径；实际路径由 `STARHUB_SERVER_PLUGIN_PATH` 指定。所有运行 Moderation API 或其 Temporal Worker 的实例都需要访问相同的、不可变的插件版本目录。运维应在这些实例的服务环境中设置 `PATH=/tmp/modelscan-plugin-venv/bin:$PATH`。插件只执行固定的 `modelscan` 命令，不接受管理员提供的命令路径。插件进程由 Moderation 启动，无需单独启动 HTTP 服务。

管理员向 `POST /api/v1/admin/security-scan/plugins` 发送名称、版本、二进制 SHA-256、选项和密钥环境变量引用，例如：

```json
{
  "name": "modelscan",
  "version": "sample-1",
  "sha256": "<64位十六进制摘要>",
  "options": {},
  "secret_env": []
}
```

登记时 Moderation 校验路径和摘要，启动插件并读取能力；随后管理员可检测并启用。`POST /api/v1/admin/security-scan/plugins/download` 只把仓库包放入暂存区；真实验签尚未实现，下载件暂存于 `OPENCSG_SECURITY_SCAN_PLUGIN_STAGING_PATH`，不可启用。

这是协议和本地验证样例：任务状态仅在内存中，重启会丢失；取消只阻止发布待完成的结果。生产适配器需要持久任务、进程隔离、资源限制、运行监控及真正中止工具进程的能力。样例只声明 `.pkl`，不能代表整仓扫描。
