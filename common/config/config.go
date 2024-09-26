package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Saas          bool   `envconfig:"STARHUB_SERVER_SAAS" default:"false"`
	InstanceID    string `envconfig:"STARHUB_SERVER_INSTANCE_ID"`
	EnableSwagger bool   `envconfig:"STARHUB_SERVER_ENABLE_SWAGGER" default:"false"`
	APIToken      string `envconfig:"STARHUB_SERVER_API_TOKEN" default:"0c11e6e4f2054444374ba3f0b70de4145935a7312289d404814cd5907c6aa93cc65cd35dbf94e04c13a3dedbf51f1694de84240c8acb7238b54a2c3ac8e87c59"`
	// enable if you want to acess csghub through https, especially for space rproxy
	EnableHTTPS bool `envconfig:"STARHUB_SERVER_ENABLE_HTTPS" default:"false"`

	APIServer struct {
		Port         int    `envconfig:"STARHUB_SERVER_SERVER_PORT" default:"8080"`
		PublicDomain string `envconfig:"STARHUB_SERVER_PUBLIC_DOMAIN" default:"http://localhost:8080"`
		SSHDomain    string `envconfig:"STARHUB_SERVER_SSH_DOMAIN" default:"ssh://git@localhost:2222"`
	}

	Mirror struct {
		URL              string `envconfig:"STARHUB_SERVER_MIRROR_URL" default:"http://localhost:8085"`
		Token            string `envconfig:"STARHUB_SERVER_MIRROR_Token" default:""`
		Port             int    `envconfig:"STARHUB_SERVER_MIRROR_PORT" default:"8085"`
		SessionSecretKey string `envconfig:"STARHUB_SERVER_MIRROR_SESSION_SECRET_KEY" default:"mirror"`
		WorkerNumber     int    `envconfig:"STARHUB_SERVER_MIRROR_WORKER_NUMBER" default:"5"`
	}

	DocsHost string `envconfig:"STARHUB_SERVER_SERVER_DOCS_HOST" default:"http://localhost:6636"`

	Database struct {
		Driver   string `envconfig:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN      string `envconfig:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable"`
		TimeZone string `envconfig:"STARHUB_DATABASE_TIMEZONE" default:"Asia/Shanghai"`
	}

	Redis struct {
		Endpoint           string `envconfig:"STARHUB_SERVER_REDIS_ENDPOINT"              default:"localhost:6379"`
		MaxRetries         int    `envconfig:"STARHUB_SERVER_REDIS_MAX_RETRIES"           default:"3"`
		MinIdleConnections int    `envconfig:"STARHUB_SERVER_REDIS_MIN_IDLE_CONNECTIONS"  default:"0"`
		User               string `envconfig:"STARHUB_SERVER_REDIS_USER"`
		Password           string `envconfig:"STARHUB_SERVER_REDIS_PASSWORD"`
		SentinelMode       bool   `envconfig:"STARHUB_SERVER_REDIS_USE_SENTINEL"          default:"false"`
		SentinelMaster     string `envconfig:"STARHUB_SERVER_REDIS_SENTINEL_MASTER"`
		SentinelEndpoint   string `envconfig:"STARHUB_SERVER_REDIS_SENTINEL_ENDPOINT"`
	}

	GitServer struct {
		URL        string `envconfig:"STARHUB_SERVER_GITSERVER_URL"    default:"http://localhost:3000"`
		Type       string `envconfig:"STARHUB_SERVER_GITSERVER_TYPE"    default:"gitea"`
		Host       string `envconfig:"STARHUB_SERVER_GITSERVER_HOST"       default:"http://localhost:3000"`
		SecretKey  string `envconfig:"STARHUB_SERVER_GITSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username   string `envconfig:"STARHUB_SERVER_GITSERVER_USERNAME" default:"root"`
		Password   string `envconfig:"STARHUB_SERVER_GITSERVER_PASSWORD" default:"password123"`
		TimtoutSEC int    `envconfig:"STARHUB_SERVER_GITSERVER_TIMEOUT_SEC" default:"5"`
	}

	GitalyServer struct {
		Address   string `envconfig:"STARHUB_SERVER_GITALY_SERVER_SOCKET" default:"tcp://localhost:9999"`
		Storge    string `envconfig:"STARHUB_SERVER_GITALY_STORGE" default:"default"`
		Token     string `envconfig:"STARHUB_SERVER_GITALY_TOKEN" default:"abc123secret"`
		JWTSecret string `envconfig:"STARHUB_SERVER_GITALY_JWT_SECRET" default:"signing-key"`
	}

	MirrorServer struct {
		Enable    bool   `envconfig:"STARHUB_SERVER_MIRRORSERVER_ENABLE" default:"true"`
		URL       string `envconfig:"STARHUB_SERVER_MIRRORSERVER_URL"    default:"http://localhost:3001"`
		Type      string `envconfig:"STARHUB_SERVER_MIRRORSERVER_TYPE"    default:"gitea"`
		Host      string `envconfig:"STARHUB_SERVER_MIRRORSERVER_HOST"       default:"http://localhost:3001"`
		SecretKey string `envconfig:"STARHUB_SERVER_MIRRORSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username  string `envconfig:"STARHUB_SERVER_MIRRORSERVER_USERNAME" default:"root"`
		Password  string `envconfig:"STARHUB_SERVER_MIRRORSERVER_PASSWORD" default:"password123"`
	}

	Frontend struct {
		URL string `envconfig:"STARHUB_SERVER_FRONTEND_URL" default:"https://opencsg.com"`
	}

	S3 struct {
		SSL             bool   `envconfig:"STARHUB_SERVER_S3_SSL" default:"false"`
		AccessKeyID     string `envconfig:"STARHUB_SERVER_S3_ACCESS_KEY_ID"`
		AccessKeySecret string `envconfig:"STARHUB_SERVER_S3_ACCESS_KEY_SECRET"`
		Region          string `envconfig:"STARHUB_SERVER_S3_REGION"`
		Endpoint        string `envconfig:"STARHUB_SERVER_S3_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
		Bucket          string `envconfig:"STARHUB_SERVER_S3_BUCKET" default:"opencsg-test"`
		EnableSSL       bool   `envconfig:"STARHUB_SERVER_S3_ENABLE_SSL" default:"false"`
	}

	SensitiveCheck struct {
		Enable          bool   `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE" default:"false"`
		AccessKeyID     string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID"`
		AccessKeySecret string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET"`
		Region          string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_REGION"`
		Endpoint        string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
		EnableSSL       bool   `envconfig:"STARHUB_SERVER_S3_ENABLE_SSH" default:"true"`
	}

	JWT struct {
		SigningKey string `envconfig:"STARHUB_JWT_SIGNING_KEY" default:"signing-key"`
		ValidHour  int    `envconfig:"STARHUB_JWT_VALIDATE_HOUR" default:"24"`
	}

	Inference struct {
		ServerAddr string `envconfig:"STARHUB_SERVER_INFERENCE_SERVER_ADDR" default:"http://localhost:8000"`
	}

	Space struct {
		BuilderEndpoint string `envconfig:"STARHUB_SERVER_SPACE_BUILDER_ENDPOINT" default:"http://localhost:8081"`
		// base url for space api running in k8s cluster
		RunnerEndpoint   string `envconfig:"STARHUB_SERVER_SPACE_RUNNER_ENDPOINT" default:"http://localhost:8082"`
		RunnerServerPort int    `envconfig:"STARHUB_SERVER_SPACE_RUNNER_SERVER_PORT" default:"8082"`

		// the internal root domain will be proxied to, should be internal access only
		InternalRootDomain string `envconfig:"STARHUB_SERVER_INTERNAL_ROOT_DOMAIN" default:"internal.example.com"`
		// the public root domain will be proxied from
		PublicRootDomain string `envconfig:"STARHUB_SERVER_PUBLIC_ROOT_DOMAIN" default:"public.example.com"`
		DockerRegBase    string `envconfig:"STARHUB_SERVER_DOCKER_REG_BASE" default:"registry.cn-beijing.aliyuncs.com/opencsg_public/"`
		ImagePullSecret  string `envconfig:"STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET" default:"opencsg-pull-secret"`
		// reverse proxy listening port
		RProxyServerPort int `envconfig:"STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT" default:"8083"`
		// secret key for session encryption
		SessionSecretKey   string `envconfig:"STARHUB_SERVER_SPACE_SESSION_SECRET_KEY" default:"secret"`
		DeployTimeoutInMin int    `envconfig:"STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES" default:"30"`
		// gpu model label
		GPUModelLabel            string `envconfig:"STARHUB_SERVER_GPU_MODEL_LABEL" default:"aliyun.accelerator/nvidia_name"`
		ReadnessDelaySeconds     int    `envconfig:"STARHUB_SERVER_READNESS_DELAY_SECONDS" default:"120"`
		ReadnessPeriodSeconds    int    `envconfig:"STARHUB_SERVER_READNESS_PERIOD_SECONDS" default:"10"`
		ReadnessFailureThreshold int    `envconfig:"STARHUB_SERVER_READNESS_FAILURE_THRESHOLD" default:"3"`
	}

	Model struct {
		DeployTimeoutInMin int    `envconfig:"STARHUB_SERVER_MODEL_DEPLOY_TIMEOUT_IN_MINUTES" default:"60"`
		DownloadEndpoint   string `envconfig:"STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT" default:"https://hub.opencsg.com"`
		DockerRegBase      string `envconfig:"STARHUB_SERVER_MODEL_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/public/"`
	}
	// send events
	Event struct {
		SyncInterval int `envconfig:"STARHUB_SERVER_SYNC_IN_MINUTES" default:"1"`
	}

	Casdoor struct {
		ClientID         string `envconfig:"STARHUB_SERVER_CASDOOR_CLIENT_ID" default:"client_id"`
		ClientSecret     string `envconfig:"STARHUB_SERVER_CASDOOR_CLIENT_SECRET" default:"client_secret"`
		Endpoint         string `envconfig:"STARHUB_SERVER_CASDOOR_ENDPOINT" default:"http://localhost:80"`
		Certificate      string `envconfig:"STARHUB_SERVER_CASDOOR_CERTIFICATE" default:"/etc/casdoor/certificate.pem"`
		OrganizationName string `envconfig:"STARHUB_SERVER_CASDOOR_ORGANIZATION_NAME" default:"opencsg"`
		ApplicationName  string `envconfig:"STARHUB_SERVER_CASDOOR_APPLICATION_NAME" default:"opencsg"`
	}

	Nats struct {
		URL                      string `envconfig:"OPENCSG_ACCOUNTING_NATS_URL" default:"nats://account:g98dc5FA8v4J7ck90w@natsmaster:4222"`
		MsgFetchTimeoutInSEC     int    `envconfig:"OPENCSG_ACCOUNTING_MSG_FETCH_TIMEOUTINSEC" default:"5"`
		MeterRequestSubject      string `envconfig:"OPENCSG_ACCOUNTING_METER_EVENT_SUBJECT" default:"accounting.metering.>"`
		MeterDurationSendSubject string `envconfig:"STARHUB_SERVER_METER_DURATION_SEND_SUBJECT" default:"accounting.metering.duration"`
		MeterTokenSendSubject    string `envconfig:"STARHUB_SERVER_METER_TOKEN_SEND_SUBJECT" default:"accounting.metering.token"`
		MeterQuotaSendSubject    string `envconfig:"STARHUB_SERVER_METER_QUOTA_SEND_SUBJECT" default:"accounting.metering.quota"`
	}

	Accounting struct {
		Host string `envconfig:"OPENCSG_ACCOUNTING_SERVER_HOST" default:"http://localhost"`
		Port int    `envconfig:"OPENCSG_ACCOUNTING_SERVER_PORT" default:"8086"`
	}

	User struct {
		Host                     string `envconfig:"OPENCSG_USER_SERVER_HOST" default:"http://localhost"`
		Port                     int    `envconfig:"OPENCSG_USER_SERVER_PORT" default:"8088"`
		SigninSuccessRedirectURL string `envconfig:"OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL" default:"http://localhost:3000/server/callback"`
	}

	MultiSync struct {
		SaasAPIDomain  string `envconfig:"OPENCSG_SAAS_API_DOMAIN" default:"https://hub.opencsg.com"`
		SaasSyncDomain string `envconfig:"OPENCSG_SAAS_SYNC_DOMAIN" default:"https://sync.opencsg.com"`
		Enabled        bool   `envconfig:"STARHUB_SERVER_MULTI_SYNC_ENABLED" default:"false"`
	}

	Telemetry struct {
		Enable    bool   `envconfig:"STARHUB_SERVER_TELEMETRY_ENABLE" default:"true"`
		ReportURL string `envconfig:"STARHUB_SERVER_TELEMETRY_URL" default:"http://hub.opencsg.com/api/v1/telemetry"`
	}
}

func LoadConfig() (cfg *Config, err error) {
	cfg = new(Config)
	err = envconfig.Process("", cfg)
	if err != nil {
		return
	}

	return
}
