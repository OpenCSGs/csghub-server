```mermaid
sequenceDiagram
    participant Client as Client
    participant MultiSync as MultiSyncComponent
    participant MultiSyncStore as MultiSyncStore
    participant SyncClient as SyncClient
    participant RemoteMultiSync as RemoteMultiSyncService
    participant RepoStore as RepoStore
    participant TagStore as TagStore
    participant FileStore as FileStore
    participant ModelStore as ModelStore
    participant DatasetStore as DatasetStore
    participant UserStore as UserStore
    participant GitServer as GitServer
    participant Accounting as AccountingService

    Client->>MultiSync: SyncAsClient(ctx, sc)
    
    MultiSync->>MultiSyncStore: GetLatest(ctx)
    MultiSyncStore-->>MultiSync: latest version
    
    loop while hasMore
        MultiSync->>SyncClient: Latest(ctxWithTimeout, currentVersion)
        SyncClient->>RemoteMultiSync: GET /api/v1/sync/versions?since=currentVersion
        RemoteMultiSync-->>SyncClient: sync versions response
        SyncClient-->>MultiSync: sync versions
        
        loop for each version
            MultiSync->>MultiSyncStore: createLocalSyncVersion(v)
        end
        
        alt has more versions
            MultiSync->>MultiSyncStore: GetAfterDistinct(v.Version)
            MultiSyncStore-->>MultiSync: distinct versions
            
            loop for each distinct version
                alt ModelRepo
                    MultiSync->>SyncClient: ModelInfo(ctx, sv)
                    SyncClient->>RemoteMultiSync: GET /api/v1/models/{path}
                    RemoteMultiSync-->>SyncClient: model info
                    SyncClient-->>MultiSync: model info
                    
                    MultiSync->>SyncClient: ReadMeData(ctx, sv)
                    SyncClient->>RemoteMultiSync: GET /api/v1/models/{path}/readme
                    RemoteMultiSync-->>SyncClient: readme data
                    SyncClient-->>MultiSync: readme data
                    
                    MultiSync->>MultiSync: createLocalModel()
                else DatasetRepo
                    MultiSync->>SyncClient: DatasetInfo(ctx, sv)
                    SyncClient->>RemoteMultiSync: GET /api/v1/datasets/{path}
                    RemoteMultiSync-->>SyncClient: dataset info
                    SyncClient-->>MultiSync: dataset info
                    
                    MultiSync->>SyncClient: ReadMeData(ctx, sv)
                    SyncClient->>RemoteMultiSync: GET /api/v1/datasets/{path}/readme
                    RemoteMultiSync-->>SyncClient: readme data
                    SyncClient-->>MultiSync: readme data
                    
                    MultiSync->>MultiSync: createLocalDataset()
                end
            end
        end
    end
    
    note right of MultiSync: Local Creation Process
    rect rgb(240, 240, 240)
        MultiSync->>Accounting: Check quota
        Accounting-->>MultiSync: quota status
        
        MultiSync->>UserStore: FindByUsername(namespace)
        alt user not exists
            MultiSync->>GitServer: CreateUser()
            GitServer-->>MultiSync: user info
            MultiSync->>UserStore: Create(user, namespace)
        end
        
        MultiSync->>RepoStore: UpdateOrCreateRepo(dbRepo)
        
        alt has tags
            MultiSync->>TagStore: FindOrCreate(tag)
            MultiSync->>RepoStore: DeleteAllTags(repoID)
            MultiSync->>RepoStore: BatchCreateRepoTags(repoTags)
        end
        
        MultiSync->>RepoStore: DeleteAllFiles(repoID)
        MultiSync->>SyncClient: FileList(ctx, s)
        SyncClient->>RemoteMultiSync: GET /api/v1/repos/{path}/files
        RemoteMultiSync-->>SyncClient: files list
        SyncClient-->>MultiSync: files list
        MultiSync->>FileStore: BatchCreate(dbFiles)
        
        alt is ModelRepo
            MultiSync->>ModelStore: CreateIfNotExist(dbModel)
        else is DatasetRepo
            MultiSync->>DatasetStore: CreateIfNotExist(dbDataset)
        end
    end
    
    MultiSync-->>Client: nil (success)
```

## API 文档: /api/v1/sync/versions

### 接口描述
获取自指定版本以来的同步版本信息

### 请求方法
GET

### 请求路径
`/api/v1/sync/versions`

### 请求参数
| 参数名 | 类型 | 必填 | 描述 |
|-------|------|------|------|
| cur | string | 是 | 当前版本号 |
| current_user | string | 是 | 当前用户标识 |

### 响应格式
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "versions": [
      {
        "version": 0,
        "source_id": 0,
        "repo_path": "string",
        "repo_type": "string",
        "last_modify_time": "string",
        "change_log": "string"
      }
    ],
    "has_more": true
  }
}
```

### 字段说明
- `versions`: 版本变更列表
  - `version`: 版本号 (int64)
  - `source_id`: 数据源ID (0: opencsg, 1: HF)
  - `repo_path`: 仓库路径
  - `repo_type`: 仓库类型
  - `last_modify_time`: 最后修改时间
  - `change_log`: 变更日志
- `has_more`: 是否还有更多数据

### 错误码
| 错误码 | 描述 |
|-------|------|
| 400 | 无效的请求参数 |
| 401 | 未授权访问 |
| 500 | 服务器内部错误 |