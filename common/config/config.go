package config

import (
	"context"
	"log/slog"
	"os"
	"reflect"

	"github.com/mcuadros/go-defaults"
	"github.com/naoina/toml"
	"github.com/sethvargo/go-envconfig"
)

var configFile = ""

type Config struct {
	Saas          bool   `env:"STARHUB_SERVER_SAAS" default:"false"`
	InstanceID    string `env:"STARHUB_SERVER_INSTANCE_ID"`
	EnableSwagger bool   `env:"STARHUB_SERVER_ENABLE_SWAGGER" default:"false"`
	APIToken      string `env:"STARHUB_SERVER_API_TOKEN" default:"0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59"`
	// enable if you want to acess csghub through https, especially for space rproxy
	EnableHTTPS bool   `env:"STARHUB_SERVER_ENABLE_HTTPS" default:"false"`
	DocsHost    string `env:"STARHUB_SERVER_SERVER_DOCS_HOST" default:"http://localhost:6636"`
	//the master host
	IsMasterHost bool `env:"STARHUB_SERVER_IS_MASTER_HOST" default:"true"`

	APIServer struct {
		Port         int    `env:"STARHUB_SERVER_SERVER_PORT" default:"8080"`
		PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN" default:"http://localhost:8080"`
		SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN" default:"git@localhost:2222"`
	}

	Mirror struct {
		URL              string `env:"STARHUB_SERVER_MIRROR_URL" default:"http://localhost:8085"`
		Token            string `env:"STARHUB_SERVER_MIRROR_Token" default:""`
		Port             int    `env:"STARHUB_SERVER_MIRROR_PORT" default:"8085"`
		SessionSecretKey string `env:"STARHUB_SERVER_MIRROR_SESSION_SECRET_KEY" default:"mirror"`
		WorkerNumber     int    `env:"STARHUB_SERVER_MIRROR_WORKER_NUMBER" default:"5"`
	}

	Database struct {
		Driver   string `env:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN      string `env:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable"`
		TimeZone string `env:"STARHUB_DATABASE_TIMEZONE" default:"Asia/Shanghai"`
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
		Storage   string `env:"STARHUB_SERVER_GITALY_STORGE" default:"default"`
		Token     string `env:"STARHUB_SERVER_GITALY_TOKEN" default:"abc123secret"`
		JWTSecret string `env:"STARHUB_SERVER_GITALY_JWT_SECRET" default:"signing-key"`
	}

	MirrorServer struct {
		Enable    bool   `env:"STARHUB_SERVER_MIRRORSERVER_ENABLE" default:"false"`
		URL       string `env:"STARHUB_SERVER_MIRRORSERVER_URL" default:"http://localhost:3001"`
		Type      string `env:"STARHUB_SERVER_MIRRORSERVER_TYPE" default:"gitea"`
		Host      string `env:"STARHUB_SERVER_MIRRORSERVER_HOST" default:"http://localhost:3001"`
		SecretKey string `env:"STARHUB_SERVER_MIRRORSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username  string `env:"STARHUB_SERVER_MIRRORSERVER_USERNAME" default:"root"`
		Password  string `env:"STARHUB_SERVER_MIRRORSERVER_PASSWORD" default:"password123"`
	}

	Frontend struct {
		URL string `env:"STARHUB_SERVER_FRONTEND_URL" default:"https://opencsg.com"`
	}

	S3 struct {
		SSL             bool   `env:"STARHUB_SERVER_S3_SSL" default:"false"`
		AccessKeyID     string `env:"STARHUB_SERVER_S3_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"STARHUB_SERVER_S3_ACCESS_KEY_SECRET"`
		Region          string `env:"STARHUB_SERVER_S3_REGION"`
		Endpoint        string `env:"STARHUB_SERVER_S3_ENDPOINT" default:"localhost:9000"`
		//for better performance of LFS downloading from s3. (can ignore if S3.Endpoint is alreay an internal domain or ip address)
		InternalEndpoint string `env:"STARHUB_SERVER_S3_INTERNAL_ENDPOINT" default:""`
		Bucket           string `env:"STARHUB_SERVER_S3_BUCKET" default:"opencsg-test"`
		EnableSSL        bool   `env:"STARHUB_SERVER_S3_ENABLE_SSL" default:"false"`
		BucketLookup     string `env:"STARHUB_SERVER_S3_BUCKET_LOOKUP" default:"auto"`
	}

	SensitiveCheck struct {
		Enable          bool   `env:"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE" default:"false"`
		AccessKeyID     string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET"`
		Region          string `env:"STARHUB_SERVER_SENSITIVE_CHECK_REGION"`
		MaxImageCount   int    `env:"STARHUB_SERVER_SENSITIVE_CHECK_MAX_IMAGE_COUNT" default:"10"`
		Endpoint        string `env:"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
		EnableSSL       bool   `env:"STARHUB_SERVER_S3_ENABLE_SSH" default:"true"`
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
		PublicRootDomain string `env:"STARHUB_SERVER_PUBLIC_ROOT_DOMAIN" default:"public.example.com"`
		DockerRegBase    string `env:"STARHUB_SERVER_DOCKER_REG_BASE" default:"registry.cn-beijing.aliyuncs.com/opencsg_public/"`
		ImagePullSecret  string `env:"STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET" default:"opencsg-pull-secret"`
		// reverse proxy listening port
		RProxyServerPort int `env:"STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT" default:"8083"`
		// secret key for session encryption
		SessionSecretKey   string `env:"STARHUB_SERVER_SPACE_SESSION_SECRET_KEY" default:"secret"`
		DeployTimeoutInMin int    `env:"STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES" default:"30"`
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
		DockerRegBase           string `env:"STARHUB_SERVER_MODEL_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/public/"`
		NimDockerSecretName     string `env:"STARHUB_SERVER_MODEL_NIM_DOCKER_SECRET_NAME" default:"ngc-secret"`
		NimNGCSecretName        string `env:"STARHUB_SERVER_MODEL_NIM_NGC_SECRET_NAME" default:"nvidia-nim-secrets"`
		MinContextForEstimation int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_ESTIMATION" default:"8192"`
		MinContextForFinetune   int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_FINETUNE" default:"512"`
	}
	// send events
	Event struct {
		SyncInterval int `env:"STARHUB_SERVER_SYNC_IN_MINUTES" default:"1"`
	}

	Casdoor struct {
		ClientID         string `env:"STARHUB_SERVER_CASDOOR_CLIENT_ID" default:"client_id"`
		ClientSecret     string `env:"STARHUB_SERVER_CASDOOR_CLIENT_SECRET" default:"client_secret"`
		Endpoint         string `env:"STARHUB_SERVER_CASDOOR_ENDPOINT" default:"http://localhost:80"`
		Certificate      string `env:"STARHUB_SERVER_CASDOOR_CERTIFICATE" default=:"etc/casdoor/certificate.pem"`
		OrganizationName string `env:"STARHUB_SERVER_CASDOOR_ORGANIZATION_NAME" default:"opencsg"`
		ApplicationName  string `env:"STARHUB_SERVER_CASDOOR_APPLICATION_NAME" default:"opencsg"`
	}

	Nats struct {
		URL                      string `env:"OPENCSG_ACCOUNTING_NATS_URL" default:"nats://account:g98dc5FA8v4J7ck90w@natsmaster:4222"`
		MsgFetchTimeoutInSEC     int    `env:"OPENCSG_ACCOUNTING_MSG_FETCH_TIMEOUTINSEC" default:"5"`
		MeterRequestSubject      string `env:"OPENCSG_ACCOUNTING_METER_EVENT_SUBJECT" default:"accounting.metering.>"`
		MeterDurationSendSubject string `env:"STARHUB_SERVER_METER_DURATION_SEND_SUBJECT" default:"accounting.metering.duration"`
		MeterTokenSendSubject    string `env:"STARHUB_SERVER_METER_TOKEN_SEND_SUBJECT" default:"accounting.metering.token"`
		MeterQuotaSendSubject    string `env:"STARHUB_SERVER_METER_QUOTA_SEND_SUBJECT" default:"accounting.metering.quota"`
		ServiceUpdateSubject     string `env:"STARHUB_SERVER_DEPLOY_SERVICE_SUBJECT" default:"deploy.service.update"`
	}

	Accounting struct {
		Host           string `env:"OPENCSG_ACCOUNTING_SERVER_HOST" default:"http://localhost"`
		Port           int    `env:"OPENCSG_ACCOUNTING_SERVER_PORT" default:"8086"`
		ChargingEnable bool   `env:"OPENCSG_ACCOUNTING_CHARGING_ENABLE" default:"false"`
	}

	User struct {
		Host                     string `env:"OPENCSG_USER_SERVER_HOST" default:"http://localhost"`
		Port                     int    `env:"OPENCSG_USER_SERVER_PORT" default:"8088"`
		SigninSuccessRedirectURL string `env:"OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL" default:"http://localhost:3000/server/callback"`
	}

	MultiSync struct {
		SaasAPIDomain  string `env:"OPENCSG_SAAS_API_DOMAIN" default:"https://hub.opencsg.com"`
		SaasSyncDomain string `env:"OPENCSG_SAAS_SYNC_DOMAIN" default:"https://sync.opencsg.com"`
		Enabled        bool   `env:"STARHUB_SERVER_MULTI_SYNC_ENABLED" default:"true"`
	}

	Telemetry struct {
		Enable    bool   `env:"STARHUB_SERVER_TELEMETRY_ENABLE" default:"true"`
		ReportURL string `env:"STARHUB_SERVER_TELEMETRY_URL" default:"http://hub.opencsg.com/api/v1/telemetry"`
	}

	AutoClean struct {
		Instance bool `env:"OPENCSG_AUTO_CLEANUP_INSTANCE_ENABLE" default:"false"`
	}

	Dataset struct {
		PromptMaxJsonlFileSize int64 `env:"OPENCSG_PROMPT_MAX_JSONL_FILESIZE_BYTES" default:"1048576"` // 1MB
	}

	Dataflow struct {
		Host string `env:"OPENCSG_DATAFLOW_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_DATAFLOW_SERVER_PORT" default:"8000"`
	}

	Moderation struct {
		Host string `env:"OPENCSG_MODERATION_SERVER_HOST" default:"http://localhost"`
		Port int    `env:"OPENCSG_MODERATION_SERVER_PORT" default:"8089"`
		// comma splitted, and base64 encoded
		EncodedSensitiveWords string `env:"OPENCSG_MODERATION_SERVER_ENCODED_SENSITIVE_WORDS" default:"5Lmg6L+R5bmzLHhpamlucGluZw=="`
	}

	WorkFLow struct {
		Endpoint         string `env:"OPENCSG_WORKFLOW_SERVER_ENDPOINT" default:"localhost:7233"`
		ExecutionTimeout int64  `env:"OPENCSG_WORKFLOW_EXECUTION_TIMEOUT" default:"43200"`
		TaskTimeout      int64  `env:"OPENCSG_WORKFLOW_TASK_TIMEOUT" default:"43200"`
	}

	Argo struct {
		Namespace string `env:"STARHUB_SERVER_ARGO_NAMESPACE" default:"workflows"`
		// NamespaceQuota is used to create evaluation with free of charge
		QuotaNamespace string `env:"STARHUB_SERVER_ARGO_QUOTA_NAMESPACE" default:"workflows-quota"`
		QuotaGPUNumber string `env:"STARHUB_SERVER_ARGO_QUOTA_GPU_NUMBER" default:"1"`
		//job will be deleted after JobTTL seconds once the jobs was done
		JobTTL             int    `env:"STARHUB_SERVER_ARGO_TTL" default:"120"`
		ServiceAccountName string `env:"STARHUB_SERVER_ARGO_SERVICE_ACCOUNT" default:"executor"`
		// S3PublicBucket is used to store public files, should set bucket same with portal
		S3PublicBucket string `env:"STARHUB_SERVER_ARGO_S3_PUBLIC_BUCKET"`
	}

	CronJob struct {
		SyncAsClientCronExpression   string `env:"STARHUB_SERVER_CRON_JOB_SYNC_AS_CLIENT_CRON_EXPRESSION" default:"0 * * * *"`
		CalcRecomScoreCronExpression string `env:"STARHUB_SERVER_CRON_JOB_CLAC_RECOM_SCORE_CRON_EXPRESSION" default:"0 1 * * *"`
	}

	DataViewer struct {
		Host                                    string `env:"OPENCSG_DATAVIEWER_SERVER_HOST" default:"http://localhost"`
		Port                                    int    `env:"OPENCSG_DATAVIEWER_SERVER_PORT" default:"8093"`
		MaxConcurrentActivityExecutionSize      int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_ACTIVITY_EXECUTION_SIZE" default:"5"`
		MaxConcurrentLocalActivityExecutionSize int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_LOCAL_ACTIVITY_EXECUTION_SIZE" default:"10"`
		MaxConcurrentWorkflowTaskExecutionSize  int    `env:"OPENCSG_DATAVIEWER_MAX_CONCURRENT_WORKFLOW_TASK_EXECUTION_SIZE" default:"2"`
		ActivityStartToCloseTimeout             int    `env:"OPENCSG_DATAVIEWER_ACTIVITY_START_TO_CLOSE_TIMEOUT" default:"7200"`
		ActivityMaximumAttempts                 int32  `env:"OPENCSG_DATAVIEWER_ACTIVITY_MAXIMUM_ATTEMPTS" default:"2"`
		CacheDir                                string `env:"OPENCSG_DATAVIEWER_CACHE_DIR" default=:"tmp/opencsg"`
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
		OTLPEndpoint string `env:"OPENCSG_TRACING_OTLP_ENDPOINT"`
		//Note: don't enable it unless you have no other way to collect service logs. It will leads to very high CPU usage.
		OTLPLogging bool `env:"OPENCSG_TRACING_OTLP_LOGGING"`
	}

	Git struct {
		// Timeout time(seconds) for git operations
		OperationTimeout      int    `env:"STARHUB_SERVER_GIT_OPERATION_TIMEOUT" default:"300"`
		SkipLfsFileValidation bool   `env:"STARHUB_SERVER_SKIP_LFS_FILE_VALIDATION" default:"false"`
		SignatureSecertKey    string `env:"STARHUB_SERVER_GIT_SIGNATURE_SECRET_KEY" default:"s"`
		MinMultipartSize      int64  `env:"STARHUB_SERVER_GIT_MIN_MULTIPART_SIZE" default:"52428800"`
		MaxUnLfsFileSize      int64  `env:"STARHUB_SERVER_GIT_MAX_UN_LFS_FILE_SIZE" default:"20971520"`
	}

	AIGateway struct {
		Port int `env:"OPENCSG_AIGATEWAY_PORT" default:"8094"`
	}

	Runner struct {
		ImageBuilderClusterID   string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_CLUSTER_ID" default:""`
		ImageBuilderNamespace   string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_NAMESPACE" default:"imagebuilder"`
		ImageBuilderGitImage    string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_GIT_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/alpine/git:2.36.2"`
		ImageBuilderKanikoImage string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/public/kaniko-project-executor:v1.23.2"`
		ImageBuilderJobTTL      int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_JOB_TTL" default:"120"`
		ImageBuilderStatusTTL   int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_STATUS_TTL" default:"300"`
		ImageBuilderKanikoArgs  []string `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_ARGS"`
	}

	RepoTemplate struct {
		EmptyRepoType  string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_REPO_TYPE" default:"template"`
		EmptyNameSpace string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_NAMESPACE" default:"emptynamespace"`
		EmptyRepoName  string `env:"STARHUB_SERVER_REPO_TEMPLATE_EMPTY_REPO_NAME" default:"emptyreponame"`
	}

	Prometheus struct {
		ApiAddress string `env:"STARHUB_SERVER_PROMETHEUS_API_ADDRESS" default:""`
		BasicAuth  string `env:"STARHUB_SERVER_PROMETHEUS_BASIC_AUTH" default:""`
	}
}

func SetConfigFile(file string) {
	configFile = file
}

func LoadConfig() (*Config, error) {
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
	return cfg, err
}
