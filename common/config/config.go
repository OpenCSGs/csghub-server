package config

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/mcuadros/go-defaults"
	"github.com/naoina/toml"
	"github.com/sethvargo/go-envconfig"
)

var configFile = ""

type Config struct {
	Saas          bool   `env:"STARHUB_SERVER_SAAS" default:"false"`
	Oversea       bool   `env:"STARHUB_SERVER_OVERSEA" default:"false"`
	InstanceID    string `env:"STARHUB_SERVER_INSTANCE_ID"`
	EnableSwagger bool   `env:"STARHUB_SERVER_ENABLE_SWAGGER" default:"false"`
	// enable if you want to acess csghub through https, especially for space rproxy
	EnableHTTPS bool   `env:"STARHUB_SERVER_ENABLE_HTTPS" default:"false"`
	APIToken    string `env:"STARHUB_SERVER_API_TOKEN" default:"f3a7b9c1d6e5f8e2a1b5d4f9e6a2b8d7c3a4e2b1d9f6e7a8d2c5a7b4c1e3f5b8a1d4f9b7d6e2f8a5d3b1e7f9c6a8b2d1e4f7d5b6e9f2a4b3c8e1d7f995hd82hf"`
	//the api key to call lbs service, like tencent map or gaode map
	LBSServiceKey string `env:"STARHUB_SERVER_LBS_SERVICE_KEY" default:"123456"`
	//the cdn domain for different city
	CityToCdnDomain          map[string]string `env:"STARHUB_SERVER_CITY_TO_CDN_DOMAIN" default:""`
	UniqueServiceName        string            `env:"STARHUB_SERVER_UNIQUE_SERVICE_NAME" default:""`
	ServerFailureRedirectURL string            `env:"STARHUB_SERVER_FAIL_REDIRECT_URL" default:"http://localhost:3000/errors/server-error"`
	NeedPhoneVerify          bool              `env:"STARHUB_SERVER_NEED_PHONE_VERIFY" default:"false"`

	APIServer struct {
		Port         int    `env:"STARHUB_SERVER_SERVER_PORT" default:"8080"`
		PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN" default:"http://localhost:8080"`
		SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN" default:"ssh://git@localhost:2222"`
	}

	Mirror struct {
		URL              string `env:"STARHUB_SERVER_MIRROR_URL" default:"http://localhost:8085"`
		Token            string `env:"STARHUB_SERVER_MIRROR_Token" default:""`
		Port             int    `env:"STARHUB_SERVER_MIRROR_PORT" default:"8085"`
		SessionSecretKey string `env:"STARHUB_SERVER_MIRROR_SESSION_SECRET_KEY" default:"mirror"`
		ProxyURL         string `env:"STARHUB_SERVER_MIRROR_PROXY_URL" default:""`
		WorkerNumber     int    `env:"STARHUB_SERVER_MIRROR_WORKER_NUMBER" default:"5"`
		PartSize         int    `env:"STARHUB_SERVER_MIRROR_PART_SIZE" default:"100"`
		LfsConcurrency   int    `env:"STARHUB_SERVER_MIRROR_LFS_CONCURRENCY" default:"5"`
		// The token number to add to bucket each second
		RateLimit float64 `env:"STARHUB_SERVER_MIRROR_RATE_LIMIT" default:"0.2"`
		// The capacity of token bucket
		RateBucketCapacity int   `env:"STARHUB_SERVER_MIRROR_RATE_BUCKET_CAPACITY" default:"1"`
		MaxRetryCount      int   `env:"STARHUB_SERVER_MIRROR_MAX_RETRY_COUNT" default:"3"`
		MaxDatasetRepoSize int64 `env:"STARHUB_SERVER_MIRROR_MAX_DATASET_REPO_SIZE" default:"53687091200"` // 50GB
		MaxModelRepoSize   int64 `env:"STARHUB_SERVER_MIRROR_MAX_MODEL_REPO_SIZE" default:"53687091200"`   // 50GB
	}

	DocsHost string `env:"STARHUB_SERVER_SERVER_DOCS_HOST" default:"http://localhost:6636"`

	Database struct {
		Driver              string `env:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN                 string `env:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable"`
		TimeZone            string `env:"STARHUB_DATABASE_TIMEZONE" default:"Asia/Shanghai"`
		SearchConfiguration string `env:"STARHUB_DATABASE_SEARCH_CONFIGURATION" default:"opencsgchinese"`
	}

	Redis struct {
		Endpoint           string `env:"STARHUB_SERVER_REDIS_ENDPOINT" default:"localhost:6379"`
		MaxRetries         int    `env:"STARHUB_SERVER_REDIS_MAX_RETRIES" default:"3"`
		MinIdleConnections int    `env:"STARHUB_SERVER_REDIS_MIN_IDLE_CONNECTIONS" default:"0"`
		User               string `env:"STARHUB_SERVER_REDIS_USER"`
		Password           string `env:"STARHUB_SERVER_REDIS_PASSWORD"`
		SentinelMode       bool   `env:"STARHUB_SERVER_REDIS_USE_SENTINEL" default:"false"`
		SentinelMaster     string `env:"STARHUB_SERVER_REDIS_SENTINEL_MASTER"`
		SentinelEndpoint   string `env:"STARHUB_SERVER_REDIS_SENTINEL_ENDPOINT"`
	}

	GitServer struct {
		URL        string `env:"STARHUB_SERVER_GITSERVER_URL" default:"http://localhost:3000"`
		Type       string `env:"STARHUB_SERVER_GITSERVER_TYPE" default:"gitea"`
		Host       string `env:"STARHUB_SERVER_GITSERVER_HOST" default:"http://localhost:3000"`
		SecretKey  string `env:"STARHUB_SERVER_GITSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username   string `env:"STARHUB_SERVER_GITSERVER_USERNAME" default:"root"`
		Password   string `env:"STARHUB_SERVER_GITSERVER_PASSWORD" default:"password123"`
		TimeoutSEC int    `env:"STARHUB_SERVER_GITSERVER_TIMEOUT_SEC" default:"5"`
	}

	GitalyServer struct {
		Address   string `env:"STARHUB_SERVER_GITALY_SERVER_SOCKET" default:"tcp://localhost:9999"`
		Storage   string `env:"STARHUB_SERVER_GITALY_STORAGE" default:"default"`
		Token     string `env:"STARHUB_SERVER_GITALY_TOKEN" default:"abc123secret"`
		JWTSecret string `env:"STARHUB_SERVER_GITALY_JWT_SECRET" default:"signing-key"`
	}

	MirrorServer struct {
		Enable    bool   `env:"STARHUB_SERVER_MIRRORSERVER_ENABLE" default:"true"`
		URL       string `env:"STARHUB_SERVER_MIRRORSERVER_URL" default:"http://localhost:3000"`
		Type      string `env:"STARHUB_SERVER_MIRRORSERVER_TYPE" default:"gitea"`
		Host      string `env:"STARHUB_SERVER_MIRRORSERVER_HOST" default:"http://localhost:3000"`
		SecretKey string `env:"STARHUB_SERVER_MIRRORSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username  string `env:"STARHUB_SERVER_MIRRORSERVER_USERNAME" default:"root"`
		Password  string `env:"STARHUB_SERVER_MIRRORSERVER_PASSWORD" default:"password123"`
	}

	Frontend struct {
		URL string `env:"STARHUB_SERVER_FRONTEND_URL" default:"https://opencsg.com"`
	}

	S3 struct {
		AccessKeyID     string `env:"STARHUB_SERVER_S3_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"STARHUB_SERVER_S3_ACCESS_KEY_SECRET"`
		Region          string `env:"STARHUB_SERVER_S3_REGION"`
		Endpoint        string `env:"STARHUB_SERVER_S3_ENDPOINT" default:"localhost:9000"`
		//for better performance of LFS downloading from s3. (can ignore if S3.Endpoint is alreay an internal domain or ip address)
		InternalEndpoint string `env:"STARHUB_SERVER_S3_INTERNAL_ENDPOINT"`
		Bucket           string `env:"STARHUB_SERVER_S3_BUCKET" default:"opencsg-test"`
		EnableSSL        bool   `env:"STARHUB_SERVER_S3_ENABLE_SSL" default:"false"`
		// BucketLookup type, can be "auto" "dns" or "path"
		BucketLookup string `env:"STARHUB_SERVER_S3_BUCKET_LOOKUP" default:"auto"`
		PublicBucket string `env:"STARHUB_SERVER_S3_PUBLIC_BUCKET" default:"opencsg-public-resource"`
	}

	SensitiveCheck struct {
		Enable          bool   `env:"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE" default:"false"`
		AccessKeyID     string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET"`
		Region          string `env:"STARHUB_SERVER_SENSITIVE_CHECK_REGION"`
		Endpoint        string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
		EnableSSL       bool   `env:"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE_SSL" default:"true"`
		DictDir         string `env:"STARHUB_SERVER_SENSITIVE_CHECK_DICT_DIR" default:"/starhub-bin/vocabulary"`
	}

	JWT struct {
		SigningKey string `env:"STARHUB_JWT_SIGNING_KEY" default:"signing-key"`
		ValidHour  int    `env:"STARHUB_JWT_VALIDATE_HOUR" default:"24"`
	}

	Space struct {
		BuilderEndpoint string `env:"STARHUB_SERVER_SPACE_BUILDER_ENDPOINT" default:"http://localhost:8082"`
		// base url for space api running in k8s cluster
		RunnerEndpoint   string `env:"STARHUB_SERVER_SPACE_RUNNER_ENDPOINT" default:"http://localhost:8082"`
		RunnerServerPort int    `env:"STARHUB_SERVER_SPACE_RUNNER_SERVER_PORT" default:"8082"`

		// the internal root domain will be proxied to, should be internal access only
		InternalRootDomain string `env:"STARHUB_SERVER_INTERNAL_ROOT_DOMAIN" default:"internal.example.com"`
		// the public root domain will be proxied from
		PublicRootDomain string `env:"STARHUB_SERVER_PUBLIC_ROOT_DOMAIN"`
		DockerRegBase    string `env:"STARHUB_SERVER_DOCKER_REG_BASE" default:"registry.cn-beijing.aliyuncs.com/opencsg_public/"`
		ImagePullSecret  string `env:"STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET" default:"opencsg-pull-secret"`
		// reverse proxy listening port
		RProxyServerPort int `env:"STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT" default:"8083"`
		// secret key for session encryption
		SessionSecretKey   string `env:"STARHUB_SERVER_SPACE_SESSION_SECRET_KEY" default:"secret"`
		DeployTimeoutInMin int    `env:"STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES" default:"30"`
		BuildTimeoutInMin  int    `env:"STARHUB_SERVER_SPACE_BUILD_TIMEOUT_IN_MINUTES" default:"30"`
		// gpu model label
		GPUModelLabel             string `env:"STARHUB_SERVER_GPU_MODEL_LABEL"`
		ReadinessDelaySeconds     int    `env:"STARHUB_SERVER_READINESS_DELAY_SECONDS" default:"120"`
		ReadinessPeriodSeconds    int    `env:"STARHUB_SERVER_READINESS_PERIOD_SECONDS" default:"10"`
		ReadinessFailureThreshold int    `env:"STARHUB_SERVER_READINESS_FAILURE_THRESHOLD" default:"3"`
		PYPIIndexURL              string `env:"STARHUB_SERVER_SPACE_PYPI_INDEX_URL" default:""`
		InformerSyncPeriodInMin   int    `env:"STARHUB_SERVER_SPACE_INFORMER_SYNC_PERIOD_IN_MINUTES" default:"2"`
	}

	Model struct {
		DeployTimeoutInMin      int    `env:"STARHUB_SERVER_MODEL_DEPLOY_TIMEOUT_IN_MINUTES" default:"60"`
		DownloadEndpoint        string `env:"STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT" default:"https://hub.opencsg.com"`
		DockerRegBase           string `env:"STARHUB_SERVER_MODEL_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com"`
		NimDockerSecretName     string `env:"STARHUB_SERVER_MODEL_NIM_DOCKER_SECRET_NAME" default:"ngc-secret"`
		NimNGCSecretName        string `env:"STARHUB_SERVER_MODEL_NIM_NGC_SECRET_NAME" default:"nvidia-nim-secrets"`
		MinContextForEstimation int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_ESTIMATION" default:"2048"`
		MinContextForFinetune   int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_FINETUNE" default:"512"`
	}

	Search struct {
		RepoSearchCacheTTL int `env:"STARHUB_SERVER_REPO_SEARCH_CACHE_TTL" default:"300"` // 5 min
		RepoSearchLimit    int `env:"STARHUB_SERVER_REPO_SEARCH_LIMIT" default:"2000"`
	}

	// send events
	Event struct {
		SyncInterval int `env:"STARHUB_SERVER_SYNC_IN_MINUTES" default:"1"`
	}

	SSOType string `env:"STARHUB_SERVER_SSO_TYPE" default:"casdoor"`
	Casdoor struct {
		ClientID         string `env:"STARHUB_SERVER_CASDOOR_CLIENT_ID" default:"client_id"`
		ClientSecret     string `env:"STARHUB_SERVER_CASDOOR_CLIENT_SECRET" default:"client_secret"`
		Endpoint         string `env:"STARHUB_SERVER_CASDOOR_ENDPOINT" default:"http://localhost:80"`
		Certificate      string `env:"STARHUB_SERVER_CASDOOR_CERTIFICATE" default:"/etc/casdoor/certificate.pem"`
		OrganizationName string `env:"STARHUB_SERVER_CASDOOR_ORGANIZATION_NAME" default:"opencsg"`
		ApplicationName  string `env:"STARHUB_SERVER_CASDOOR_APPLICATION_NAME" default:"opencsg"`
	}

	Paraview struct {
		Endpoint     string `env:"STARHUB_SERVER_PARAVIEW_ENDPOINT" default:"https://iam-c.paraview.cn"`
		ClientID     string `env:"STARHUB_SERVER_PARAVIEW_CLIENT_ID" default:""`
		ClientSecret string `env:"STARHUB_SERVER_PARAVIEW_CLIENT_SECRET" default:""`
		RedirectURI  string `env:"STARHUB_SERVER_PARAVIEW_REDIRECT_URI" default:"http://127.0.0.1:8080/api/v1/callback/paraview"`
		ApiKey       string `env:"STARHUB_SERVER_PARAVIEW_API_KEY" default:""`
		ApiSecret    string `env:"STARHUB_SERVER_PARAVIEW_API_SECRET" default:""`
	}

	Nats struct {
		URL                  string `env:"OPENCSG_ACCOUNTING_NATS_URL" default:"nats://account:g98dc5FA8v4J7ck90w@natsmaster:4222"`
		MsgFetchTimeoutInSEC int    `env:"OPENCSG_ACCOUNTING_MSG_FETCH_TIMEOUTINSEC" default:"5"`
	}

	Kafka struct {
		Servers string `env:"OPENCSG_SERVERS_KAFKA_SERVERS" default:""`
	}

	Accounting struct {
		Host                         string `env:"OPENCSG_ACCOUNTING_SERVER_HOST" default:"http://localhost"`
		Port                         int    `env:"OPENCSG_ACCOUNTING_SERVER_PORT" default:"8086"`
		ChargingEnable               bool   `env:"OPENCSG_ACCOUNTING_CHARGING_ENABLE" default:"false"`
		SubscriptionCronExpression   string `env:"OPENCSG_ACCOUNTING_SUBSCRIPTION_CRON_EXPRESSION" default:"*/5 * * * *"`
		ExpiredPresentCronExpression string `env:"OPENCSG_ACCOUNTING_EXPIRED_PRESENT_CRON_EXPRESSION" default:"0 0 * * *"`
		ThresholdOfStopDeploy        int    `env:"OPENCSG_ACCOUNTING_THRESHOLD_OF_STOP_DEPLOY" default:"5000"`
	}

	User struct {
		Host                           string `env:"OPENCSG_USER_SERVER_HOST" default:"http://localhost"`
		Port                           int    `env:"OPENCSG_USER_SERVER_PORT" default:"8088"`
		SigninSuccessRedirectURL       string `env:"OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL" default:"http://localhost:3000/server/callback"`
		CodeSoulerVScodeRedirectURL    string `env:"OPENCSG_USER_SERVER_CODESOULER_VSCODE_REDIRECT_URL" default:"http://127.0.0.1:37678/callback"`
		CodeSoulerJetBrainsRedirectURL string `env:"OPENCSG_USER_SERVER_CODESOULER_JETBRAINS_REDIRECT_URL" default:"http://127.0.0.1:37679/callback"`
	}

	MultiSync struct {
		SaasAPIDomain         string `env:"OPENCSG_SAAS_API_DOMAIN" default:"https://hub.opencsg.com"`
		SaasSyncDomain        string `env:"OPENCSG_SAAS_SYNC_DOMAIN" default:"https://sync.opencsg.com"`
		DefaultRepoCountLimit int64  `env:"OPENCSG_SAAS_DEFAULT_REPO_COUNT_LIMIT" default:"10"`
		// DefaultRepoSizeLimit and DefaultTrafficLimit is in Bytes
		DefaultSpeedLimit                     int64  `env:"OPENCSG_SAAS_DEFAULT_SPEED_LIMIT" default:"1024"`
		DefaultTrafficLimit                   int64  `env:"OPENCSG_SAAS_DEFAULT_TRAFFIC_LIMIT" default:"1024"`
		Enabled                               bool   `env:"STARHUB_SERVER_MULTI_SYNC_ENABLED" default:"false"`
		RefreshAccountSyncQuotaCronExpression string `env:"STARHUB_SERVER_MULTI_SYNC_REFRESH_ACCOUNT_SYNC_QUOTA_CRON_EXPRESSION" default:"0 0 * * *"`
		HTTPInsecureSkipVerify                bool   `env:"STARHUB_SERVER_MULTI_SYNC_HTTP_INSECURE_SKIP_VERIFY" default:"false"`
	}

	Telemetry struct {
		Enable    bool   `env:"STARHUB_SERVER_TELEMETRY_ENABLE" default:"true"`
		ReportURL string `env:"STARHUB_SERVER_TELEMETRY_URL" default:"http://hub.opencsg.com/api/v1/telemetry"`
	}

	Dataset struct {
		PromptMaxJsonlFileSize int64 `env:"OPENCSG_PROMPT_MAX_JSONL_FILESIZE_BYTES" default:"1048576"` // 1MB
		// allow normal user crreate public dataset, or change private dataset to public
		AllowCreatePublicDataset bool `env:"OPENCSG_ALLOW_CREATE_PUBLIC_DATASET" default:"true"`
	}

	Dataflow struct {
		Host string `env:"OPENCSG_DATAFLOW_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_DATAFLOW_SERVER_PORT" default:"8000"`
	}

	// for csghub agents
	CSGBot struct {
		Host string `env:"OPENCSG_CSGBOT_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_CSGBOT_SERVER_PORT" default:"8070"`
	}

	Moderation struct {
		Host                     string `env:"OPENCSG_MODERATION_SERVER_HOST" default:"http://localhost"`
		Port                     int    `env:"OPENCSG_MODERATION_SERVER_PORT" default:"8089"`
		RepoFileCheckConcurrency int    `env:"OPENCSG_MODERATION_SERVER_REPO_FILE_CHECK_CONCURRENCY" default:"10"`
	}

	WorkFLow struct {
		Endpoint         string `env:"OPENCSG_WORKFLOW_SERVER_ENDPOINT" default:"localhost:7233"`
		ExecutionTimeout int64  `env:"OPENCSG_WORKFLOW_EXECUTION_TIMEOUT" default:"43200"`
		TaskTimeout      int64  `env:"OPENCSG_WORKFLOW_TASK_TIMEOUT" default:"43200"`
	}

	License struct {
		PublicKeyFile  string `env:"OPENCSG_LICENSE_PUBLIC_KEY_FILE" default:"/starhub-bin/enterprise/public_key_ee.pem"`
		PrivateKeyFile string `env:"OPENCSG_LICENSE_PRIVATE_KEY_FILE" default:"/starhub-bin/enterprise/private_key_ee.pem"`
	}

	Argo struct {
		QuotaGPUNumber string `env:"STARHUB_SERVER_ARGO_QUOTA_GPU_NUMBER" default:"1"`
		//job will be deleted after JobTTL seconds once the jobs was done
		JobTTL             int    `env:"STARHUB_SERVER_ARGO_TTL" default:"120"`
		ServiceAccountName string `env:"STARHUB_SERVER_ARGO_SERVICE_ACCOUNT" default:"executor"`
		// S3PublicBucket is used to store public files, should set bucket same with portal
		S3PublicBucket string `env:"STARHUB_SERVER_ARGO_S3_PUBLIC_BUCKET"`
	}

	Payment struct {
		WXAppId                            string `env:"STARHUB_SERVER_WXAPPID"`
		WXMchId                            string `env:"STARHUB_SERVER_WXMCHID"`
		WXMchAPIv3Key                      string `env:"STARHUB_SERVER_WXMCHAPIV3KEY"`
		WXMchCertificateSerialNumber       string `env:"STARHUB_SERVER_WXMCH_CERTIFICATE_SERIAL_NUMBER"`
		WXMchCertificatePrivateKeyFilePath string `env:"STARHUB_SERVER_WXMCH_CERTIFICATE_PRIVATE_KEY_FILE_PATH"`

		AlipayAppId      string `env:"STARHUB_SERVER_ALIPAY_APPID"`
		AlipayPrivateKey string `env:"STARHUB_SERVER_ALIPAY_PRIVATE_KEY"`
		AlipayPublicKey  string `env:"STARHUB_SERVER_ALIPAY_PUBLIC_KEY"`

		IsProd           bool   `env:"OPENCSG_PAYMENT_SERVER_IS_PROD" default:"false"`
		PaymentExpireIn  int    `env:"OPENCSG_PAYMENT_EXPIRE_IN" default:"300"`
		WXPayNotifyPath  string `env:"STARHUB_SERVER_WXPAYNOTIFY_PATH" default:"/api/v1/payment/wechat/notify"`
		AlipayNotifyPath string `env:"STARHUB_SERVER_ALIPAYNOTIFY_PATH" default:"/api/v1/payment/alipay/notify"`
		Host             string `env:"OPENCSG_PAYMENT_SERVER_HOST" default:"http://localhost"`
		Port             int    `env:"OPENCSG_PAYMENT_SERVER_PORT" default:"8090"`
		Bucket           string `env:"OPENCSG_PAYMENT_BILL_BUCKET" default:"opencsg-billing-stg"`

		DownLoadBillCronExpression string `env:"STARHUB_SERVER_CRON_JOB_DOWNLOAD_BILL_CRON_EXPRESSION" default:"10 10 * * *"`

		StripePublishableKey string `env:"STARHUB_SERVER_STRIPE_PUBLISHABLE_KEY"`
		StripeSecretKey      string `env:"STARHUB_SERVER_STRIPE_SECRET_KEY"`
		StripeWebhookSecret  string `env:"STARHUB_SERVER_STRIPE_WEBHOOK_SECRET"`
	}

	RepoSync struct {
		Port int `env:"STARHUB_SERVER_REPO_SYNC_PORT" default:"8091"`
	}

	LfsSync struct {
		Host string `env:"STARHUB_SERVER_LFS_SYNC_HOST" default:"http://localhost"`
		Port int    `env:"STARHUB_SERVER_LFS_SYNC_PORT" default:"8092"`
	}

	CronJob struct {
		SyncAsClientCronExpression               string `env:"STARHUB_SERVER_CRON_JOB_SYNC_AS_CLIENT_CRON_EXPRESSION" default:"0 * * * *"`
		CalcRecomScoreCronExpression             string `env:"STARHUB_SERVER_CRON_JOB_CLAC_RECOM_SCORE_CRON_EXPRESSION" default:"0 1 * * *"`
		MirrorCronExpression                     string `env:"STARHUB_SERVER_CRON_JOB_MIRROR_CRON_EXPRESSION" default:"* * * * *"`
		PublicModelRepoCronExpression            string `env:"STARHUB_SERVER_CRON_JOB_PUBLIC_MODEL_REPO_CRON_EXPRESSION" default:"* * * * *"`
		GlamaCrawlCronExpression                 string `env:"STARHUB_SERVER_CRON_JOB_PUBLIC_MODEL_REPO_CRON_EXPRESSION" default:"0 1 * * *"`
		MakeStatSnapshotCronExpression           string `env:"STARHUB_SERVER_CRON_JOB_MAKE_STAT_SNAPSHOT_CRON_EXPRESSION" default:"1 0 * * *"`
		SendWeeklyRechargesMailCronExpression    string `env:"STARHUB_SERVER_CRON_JOB_MAKE_WEEKLY_RECHARGES_CRON_EXPRESSION" default:"0 0 * * 1"`
		IncreaseMultisyncRepoLimitCronExpression string `env:"STARHUB_SERVER_CRON_JOB_INCREASE_MULTISYNC_REPO_LIMIT_CRON_EXPRESSION" default:"0 0 * * *"`
		MigrateRepoPathCronExpression            string `env:"STARHUB_SERVER_CRON_JOB_MIGRATE_REPO_PATH_CRON_EXPRESSION" default:"* 16-20 * * *"`
		DeletePendingDeletionCronExpression      string `env:"STARHUB_SERVER_CRON_JOB_DELETE_PENDING_DELETION_CRON_EXPRESSION" default:"0 16-20 * * *"`
		ReleaseInvitationCreditCronExpression    string `env:"STARHUB_SERVER_CRON_JOB_RELEASE_INVITATION_CREDIT_CRON_EXPRESSION" default:"0 0 5 * *"`
		MCPInspectCronExpression                 string `env:"STARHUB_SERVER_CRON_JOB_MCP_INSPECT_CRON_EXPRESSION" default:"*/5 * * * *"`
	}

	Agent struct {
		AutoHubServiceHost        string `env:"OPENCSG_AGENT_AUTOHUB_SERVICE_HOST" default:"http://internal.opencsg-stg.com:8190"`
		AgentHubServiceHost       string `env:"OPENCSG_AGENT_AGENTHUB_SERVICE_HOST" default:""`
		AgentHubServiceToken      string `env:"OPENCSG_AGENT_AGENTHUB_SERVICE_TOKEN" default:""`
		MCPInspectMaxConcurrency  int    `env:"OPENCSG_AGENT_MCP_INSPECT_MAX_CONCURRENCY" default:"50"`
		ShareSessionTokenValidDay int    `env:"STARHUB_SERVER_AGENT_SHARE_SESSION_TOKEN_VALIDATE_Day" default:"365"` // 1 year
	}

	DataViewer struct {
		Host                                    string `env:"OPENCSG_DATAVIEWER_SERVER_HOST" default:"http://localhost"`
		Port                                    int    `env:"OPENCSG_DATAVIEWER_SERVER_PORT" default:"8093"`
		MaxConcurrentActivityExecutionSize      int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_ACTIVITY_EXECUTION_SIZE" default:"5"`
		MaxConcurrentLocalActivityExecutionSize int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_LOCAL_ACTIVITY_EXECUTION_SIZE" default:"10"`
		MaxConcurrentWorkflowTaskExecutionSize  int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_WORKFLOW_TASK_EXECUTION_SIZE" default:"2"`
		ActivityStartToCloseTimeout             int    `env:"OPENCSG_DATAVIEWER_ACTIVITY_START_TO_CLOSE_TIMEOUT" default:"7200"`
		ActivityMaximumAttempts                 int32  `env:"OPENCSG_DATAVIEWER_ACTIVITY_MAXIMUM_ATTEMPTS" default:"2"`
		CacheDir                                string `env:"OPENCSG_DATAVIEWER_CACHE_DIR" default:"/tmp/opencsg"`
		DownloadLfsFile                         bool   `env:"OPENCSG_DATAVIEWER_DOWNLOAD_LFS_FILE" default:"true"`
		MaxThreadNumOfExport                    int    `env:"OPENCSG_DATAVIEWER_MAX_THREAD_NUM_OF_EXPORT" default:"8"`
		MaxConcurrentSessionExecutionSize       int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_SESSION_EXECUTION_SIZE" default:"1"`
		SessionExecutionTimeout                 int    `env:"OPENCSG_DATAVIEWER_SESSION_EXECUTION_TIMEOUT" default:"240"` // 240 mins
		ConvertLimitSize                        int64  `env:"OPENCSG_DATAVIEWER_CONVERT_LIMIT_SIZE" default:"5368709120"` // 5G
	}

	Proxy struct {
		Enable bool     `env:"STARHUB_SERVER_PROXY_ENABLE" default:"false"`
		URL    string   `env:"STARHUB_SERVER_PROXY_URL" default:""`
		Hosts  []string `env:"STARHUB_SERVER_PROXY_HOSTS, delimiter=;"`
	}

	Instrumentation struct {
		OTLPEndpoint string `env:"OPENCSG_TRACING_OTLP_ENDPOINT" default:""`
		//Note: don't enable it unless you have no other way to collect service logs. It will leads to very high CPU usage.
		OTLPLogging bool `env:"OPENCSG_TRACING_OTLP_LOGGING" default:"false"`
	}

	Git struct {
		// Timeout time(seconds) for git operations
		OperationTimeout       int    `env:"STARHUB_SERVER_GIT_OPERATION_TIMEOUT" default:"10"`
		CheckFileSizeEnabled   bool   `env:"STARHUB_SERVER_CHECK_FILE_SIZE_ENABLED" default:"true"`
		MaxUnLfsFileSize       int64  `env:"STARHUB_SERVER_GIT_MAX_UN_LFS_FILE_SIZE" default:"20971520"`
		SkipLfsFileValidation  bool   `env:"STARHUB_SERVER_SKIP_LFS_FILE_VALIDATION" default:"false"`
		SignatureSecertKey     string `env:"STARHUB_SERVER_GIT_SIGNATURE_SECRET_KEY" default:"git-secret"`
		MinMultipartSize       int64  `env:"STARHUB_SERVER_GIT_MIN_MULTIPART_SIZE" default:"52428800"`
		LfsExistsCheck         bool   `env:"STARHUB_SERVER_GIT_LFS_EXISTS_CHECK" default:"true"`
		RepoDataMigrateEnable  bool   `env:"STARHUB_SERVER_GIT_REPO_DATA_MIGRATE_ENABLE" default:"false"`
		LimitLfsFileUploadSize bool   `env:"STARHUB_SERVER_GIT_LIMIT_LFS_FILE_UPLOAD_SIZE " default:"true"`
		TreeOperationTimeout   int    `env:"STARHUB_SERVER_GIT_TREE_OPERATION_TIMEOUT" default:"3"`
	}

	AIGateway struct {
		Port int `env:"OPENCSG_AIGATEWAY_PORT" default:"8094"`
	}

	Integration struct {
		GithubToken      string `env:"STARHUB_SERVER_INTEGRATION_GITHUB_TOKEN" default:""`
		GithubAPIBaseURL string `env:"STARHUB_SERVER_INTEGRATION_GITHUB_API_BASE_URL" default:"https://api.github.com"`
	}

	RepoTemplate struct {
		EmptyRepoType  string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_REPO_TYPE" default:"template"`
		EmptyNameSpace string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_NAMESPACE" default:"emptynamespace"`
		EmptyRepoName  string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_REPO_NAME" default:"emptyreponame"`
	}

	MCPScan struct {
		Enable bool `env:"STARHUB_SERVER_MCP_SCAN_ENABLE" default:"false"`
		// tool_poison
		Plugins     []string `env:"STARHUB_SERVER_MCP_SCAN_PLUGIN_OPTIONS" default:""`
		Temperature float64  `env:"STARHUB_SERVER_MCP_SCAN_TEMPERATURE" default:"0.2"`
	}

	Notification struct {
		Port                                int    `env:"STARHUB_SERVER_NOTIFIER_PORT" default:"8095"`
		Host                                string `env:"STARHUB_SERVER_NOTIFIER_HOST" default:"http://localhost"`
		MailerHost                          string `env:"STARHUB_SERVER_MAILER_HOST" default:"smtp.qiye.aliyun.com"`
		MailerPort                          int    `env:"STARHUB_SERVER_MAILER_PORT" default:"465"`
		MailerUsername                      string `env:"STARHUB_SERVER_MAILER_USERNAME" default:""`
		MailerPassword                      string `env:"STARHUB_SERVER_MAILER_PASSWORD" default:""`
		DirectMailEnabled                   bool   `env:"STARHUB_SERVER_DIRECT_MAIL_ENABLED" default:"false"`
		DirectMailAccessKeyID               string `env:"STARHUB_SERVER_DIRECT_MAIL_ACCESS_KEY_ID" default:""`
		DirectMailAccessKeySecret           string `env:"STARHUB_SERVER_DIRECT_MAIL_ACCESS_KEY_SECRET" default:""`
		DirectMailEndpoint                  string `env:"STARHUB_SERVER_DIRECT_MAIL_ENDPOINT" default:"dm.aliyuncs.com"`
		DirectMailRegionId                  string `env:"STARHUB_SERVER_DIRECT_MAIL_REGION_ID" default:"cn-hangzhou"`
		MailerRechargeAdmin                 string `env:"STARHUB_SERVER_MAILER_RECHARGE_ADMIN" default:"contact@opencsg.com"`
		MailerWeeklyRechargesMail           string `env:"STARHUB_SERVER_MAILER_WEEKLY_RECHARGES_MAIL" default:"reconcile@opencsg.com"`
		EmailInvoiceCreatedReceiver         string `env:"STARHUB_SERVER_EMAIL_INVOICE_CREATED_RECEIVER" default:"contact@opencsg.com"`
		RepoSyncTimezone                    string `env:"STARHUB_SERVER_REPO_SYNC_TIMEZONE" default:"Asia/Shanghai"`
		RepoSyncChatID                      string `env:"STARHUB_SERVER_REPO_SYNC_CHAT_ID" default:""`
		NotificationRetryCount              int    `env:"STARHUB_SERVER_NOTIFIER_NOTIFICATION_RETRY_COUNT" default:"3"`
		BroadcastUserPageSize               int    `env:"STARHUB_SERVER_NOTIFIER_BROADCAST_USER_PAGE_SIZE" default:"100"`
		BroadcastEmailPageSize              int    `env:"STARHUB_SERVER_NOTIFIER_BROADCAST_EMAIL_PAGE_SIZE" default:"100"`
		MsgDispatcherCount                  int    `env:"STARHUB_SERVER_NOTIFIER_MSG_DISPATCHER_COUNT" default:"20"`
		HighPriorityMsgBufferSize           int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_BUFFER_SIZE" default:"100"`
		NormalPriorityMsgBufferSize         int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_BUFFER_SIZE" default:"50"`
		HighPriorityMsgAckWait              int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_ACK_WAIT" default:"60"`
		NormalPriorityMsgAckWait            int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_ACK_WAIT" default:"60"`
		HighPriorityMsgMaxDeliver           int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_MAX_DELIVER" default:"6"`
		NormalPriorityMsgMaxDeliver         int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_MAX_DELIVER" default:"6"`
		DeduplicateWindow                   int    `env:"STARHUB_SERVER_NOTIFIER_DEDUPLICATE_WINDOW" default:"5"` // 5 seconds
		SMSSign                             string `env:"STARHUB_SERVER_NOTIFIER_SMS_SIGN" default:""`
		SMSAccessKeyID                      string `env:"STARHUB_SERVER_NOTIFIER_SMS_ACCESS_KEY_ID" default:""`
		SMSAccessKeySecret                  string `env:"STARHUB_SERVER_NOTIFIER_SMS_ACCESS_KEY_SECRET" default:""`
		SMSTemplateCodeForVerifyCodeCN      string `env:"STARHUB_SERVER_NOTIFIER_SMS_TEMPLATE_CODE_FOR_VERIFY_CODE_CN" default:""`
		SMSTemplateCodeForVerifyCodeOversea string `env:"STARHUB_SERVER_NOTIFIER_SMS_TEMPLATE_CODE_FOR_VERIFY_CODE_OVERSEA" default:""`
	}

	Prometheus struct {
		ApiAddress string `env:"STARHUB_SERVER_PROMETHEUS_API_ADDRESS" default:""`
		BasicAuth  string `env:"STARHUB_SERVER_PROMETHEUS_BASIC_AUTH" default:""`
	}

	Feishu struct {
		AppID                          string `env:"STARHUB_SERVER_FEISHU_APP_ID" default:""`
		AppSecret                      string `env:"STARHUB_SERVER_FEISHU_APP_SECRET" default:""`
		BatchSendMessageCronExpression string `env:"STARHUB_SERVER_FEISHU_BATCH_SEND_MESSAGE_CRON_EXPRESSION" default:"*/10 * * * *"` // every 10 minutes
		MaxRequestContentSize          int    `env:"STARHUB_SERVER_FEISHU_MAX_REQUEST_CONTENT_SIZE" default:"20480"`                  // 20KB
		MaxDelayDuration               int    `env:"STARHUB_SERVER_FEISHU_MAX_DELAY_DURATION" default:"3600"`                         // 1 hour
		ChatIDsCacheTTL                int    `env:"STARHUB_SERVER_FEISHU_CHAT_IDS_CACHE_TTL" default:"21600"`                        // 6 hours
		MessageExpiredTTL              int    `env:"STARHUB_SERVER_FEISHU_MESSAGE_EXPIRED_TTL" default:"86400"`                       // 1 days
	}

	// K8S Cluster Configuration for Runner and Logcollectior
	Cluster struct {
		ClusterID      string `env:"STARHUB_SERVER_CLUSTER_ID" default:""`
		Region         string `env:"STARHUB_SERVER_CLUSTER_REGION" default:"region-0"`
		SpaceNamespace string `env:"STARHUB_SERVER_CLUSTER_SPACES_NAMESPACE" default:"spaces"`
		// for free saas users, limits resources
		ResourceQuotaNamespace string `env:"STARHUB_SERVER_CLUSTER_RESOURCE_QUOTA_NAMESPACE" default:"spaces"`
		QuotaName              string `env:"STARHUB_SERVER_CLUSTER_QUOTA_NAME" default:""`
	}

	Runner struct {
		PublicDomain            string   `env:"STARHUB_SERVER_RUNNER_PUBLIC_DOMAIN" default:"http://localhost:8082"`
		ImageBuilderGitImage    string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_GIT_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/alpine/git:2.36.2"`
		ImageBuilderKanikoImage string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/public/kaniko-project-executor:v1.23.2"`
		ImageBuilderJobTTL      int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_JOB_TTL" default:"120"`
		ImageBuilderStatusTTL   int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_STATUS_TTL" default:"300"`
		ImageBuilderKanikoArgs  []string `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_ARGS"`
		SystemCUDAVersion       string   `env:"STARHUB_SERVER_RUNNER_SYSTEM_CUDA_VERSION" default:""`
		// csghub server webhook endpoint
		WebHookEndpoint    string `env:"STARHUB_SERVER_RUNNER_WEBHOOK_ENDPOINT" default:"http://localhost:8080"`
		WatchConfigmapName string `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_NAME" default:"spaces-runner-config"`
		// WatchConfigmapKey           string `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_KEY" default:""`
		WatchConfigmapIntervalInSec int    `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_INTERVAL_IN_SEC" default:"60"`
		HearBeatIntervalInSec       int    `env:"STARHUB_SERVER_RUNNER_HEARTBEAT_INTERVAL_IN_SEC" default:"120"`
		RunnerNamespace             string `env:"STARHUB_SERVER_CLUSTER_RUNNER_NAMESPACE" default:"csghub"`
		PublicDockerRegBase         string `env:"STARHUB_SERVER_RUNNER_PUBLIC_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com"`
	}

	LogCollector struct {
		Port                 int    `env:"STARHUB_SERVER_LOGCOLLECTOR_PORT" default:"8096"`
		LokiURL              string `env:"STARHUB_SERVER_LOGCOLLECTOR_LOKI_URL" default:"http://localhost:3100"`
		WatchNSInterval      int    `env:"STARHUB_SERVER_LOGCOLLECTOR_WATCH_NS_INTERVAL" default:"3"`
		StreamCD             int    `env:"STARHUB_SERVER_LOGCOLLECTOR_STREAM_CD" default:"5"`
		MaxConcurrentStreams int    `env:"STARHUB_SERVER_LOGCOLLECTOR_MAX_CONCURRENT_STREAMS" default:"1000"`
		BatchSize            int    `env:"STARHUB_SERVER_LOGCOLLECTOR_BATCH_SIZE" default:"100"`
		BatchDelay           int    `env:"STARHUB_SERVER_LOGCOLLECTOR_BATCH_DELAY" default:"3"`
		DropMsgTimeout       int    `env:"STARHUB_SERVER_LOGCOLLECTOR_DROP_MSG_TIMEOUT" default:"60"`
		MaxRetries           int    `env:"STARHUB_SERVER_LOGCOLLECTOR_MAX_RETRIES" default:"3"`
		RetryInterval        int    `env:"STARHUB_SERVER_LOGCOLLECTOR_RETRY_INTERVAL" default:"1"`
		HealthInterval       int    `env:"STARHUB_SERVER_LOGCOLLECTOR_HEALTH_INTERVAL" default:"5"`
		AcceptLabelPrefix    string `env:"STARHUB_SERVER_LOGCOLLECTOR_ACCEPT_LABEL_PREFIX" default:"csghub_"`
		// the separator of log lines, default is "\\n" by client formats, "\n" sse auto newline
		LineSeparator          string `env:"STARHUB_SERVER_LOGCOLLECTOR_LINE_SEPARATOR" default:"\\n"`
		MaxStoreTimeDay        int    `env:"STARHUB_SERVER_LOGCOLLECTOR_MAX_STORE_TIME_DAY" default:"7"`
		QueryLastReportTimeout int    `env:"STARHUB_SERVER_LOGCOLLECTOR_QUERY_LAST_REPORT_TIMEOUT" default:"10"`
	}

	Temporal struct {
		MaxConcurrentActivityExecutionSize      int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_ACTIVITY_EXECUTION_SIZE" default:"5"`
		MaxConcurrentLocalActivityExecutionSize int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_LOCAL_ACTIVITY_EXECUTION_SIZE" default:"10"`
		MaxConcurrentWorkflowTaskExecutionSize  int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASK_EXECUTION_SIZE" default:"50"`
	}

	APIRateLimiter struct {
		Enable bool  `env:"STARHUB_SERVER_API_RATE_LIMITER_ENABLE" default:"false"`
		Limit  int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_LIMIT" default:"10"`
		Window int64 `env:"STARHUB_SERVER_API_RATE_LIMITER_WINDOW" default:"60"`
	}

	APILocationCheck struct {
		Enable    bool     `env:"STARHUB_SERVER_API_LOCATION_CHECK_ENABLE" default:"false"`
		WhiteList []string `env:"STARHUB_SERVER_API_LOCATION_CHECK_WHITE_LIST" default:"[China,Hong Kong,Singapore]"`
	}

	Captcha struct {
		ExceptionPaths []string `env:"STARHUB_SERVER_CAPTCHA_EXCEPTION_PATHS" default:"[/api/v1/broadcasts,/api/v1/notifications]"`
	}

	GeoIP struct {
		DBFile string `env:"STARHUB_SERVER_GEOIP_DB_FILE" default:"/starhub-bin/GeoLite2-Country.mmdb"`
	}

	Xnet struct {
		Endpoint string `env:"STARHUB_SERVER_XNET_ENDPOINT" default:"http://localhost:8097"`
		ApiKey   string `env:"STARHUB_SERVER_XNET_API_KEY" default:"f3a7b9c1d6e5f8e2a1b5d4f9e6a2b8d7c3a4e2b1d9f6e7a8d2c5a7b4c1e3f5b8a1d4f9b7d6e2f8a5d3b1e7f9c6a8b2d1e4f7d5b6e9f2a4b3c8e1d7f995hd82hf"`
	}

	StorageGateway struct {
		PartSize                int64 `env:"STARHUB_SERVER_STORAGE_GATEWAY_PART_SIZE" default:"67108864"`              // 64MB
		EnablePresignedURLProxy bool  `env:"STARHUB_SERVER_STORAGE_GATEWAY_ENABLE_PRESIGNED_URL_PROXY" default:"true"` // Enable presigned URL proxy through gateway
	}
}

func SetConfigFile(file string) {
	configFile = file
}

var globalConfig *Config
var globalConfigError error
var once sync.Once

func LoadConfig() (*Config, error) {
	once.Do(func() {
		globalConfig, globalConfigError = loadConfig()
	})
	return globalConfig, globalConfigError
}

func loadConfig() (*Config, error) {
	defer slog.Debug("end load config")
	slog.Debug("start load config")
	cfg := &Config{}
	defaults.SetDefaults(cfg)
	toml.DefaultConfig.MissingField = func(typ reflect.Type, key string) error {
		return nil
	}

	if configFile != "" {
		f, err := os.Open(configFile)
		if err != nil {
			return nil, err
		}
		err = toml.NewDecoder(f).Decode(cfg)
		if err != nil {
			return nil, err
		}

	}

	// Always read environment variables, even if a config file exists. If a config value is present in both the
	// config file and the environment, the environment value takes priority. If a config value is missing from
	// the config file, the default value (specified by the struct field's default tag) will be used.
	err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:           cfg,
		DefaultOverwrite: true,
	})
	if len(cfg.UniqueServiceName) < 1 {
		cfg.UniqueServiceName = genServiceName()
	}
	return cfg, err
}

func genServiceName() string {
	autoGenServiceName := ""
	hostname, _ := os.Hostname()
	if len(hostname) > 0 {
		autoGenServiceName = hostname
		addrs, _ := net.LookupHost(hostname)
		if len(addrs) > 0 {
			for _, ip := range addrs {
				if ip != "127.0.0.1" {
					autoGenServiceName = fmt.Sprintf("%s-%s", autoGenServiceName, ip)
					break
				}
			}
		}
	}
	if len(autoGenServiceName) < 1 {
		autoGenServiceName = fmt.Sprintf("%s_%s", "csghub-server", uuid.New().String())
	}
	slog.Debug("auto generate service name", slog.String("service_name", autoGenServiceName))
	return autoGenServiceName
}
