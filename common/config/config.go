package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	InstanceID    string `envconfig:"STARHUB_SERVER_INSTANCE_ID"`
	EnableSwagger bool   `envconfig:"STARHUB_SERVER_ENABLE_SWAGGER" default:"false"`
	APIToken      string `envconfig:"STARHUB_SERVER_API_TOKEN" default:"f3a7b9c1d6e5f8e2a1b5d4f9e6a2b8d7c3a4e2b1d9f6e7a8d2c5a7b4c1e3f5b8a1d4f9b7d6e2f8a5d3b1e7f9c6a8b2d1e4f7d5b6e9f2a4b3c8e1d7f995hd82hf"`

	APIServer struct {
		Port         int    `envconfig:"STARHUB_SERVER_SERVER_PORT" default:"8080"`
		ExternalHost string `envconfig:"STARHUB_SERVER_SERVER_EXTERNAL_HOST" default:"http://localhost"`
	}

	DocsHost string `envconfig:"STARHUB_SERVER_SERVER_DOCS_HOST" default:"http://localhost:6636"`

	Database struct {
		Driver   string `envconfig:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN      string `envconfig:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable"`
		TimeZone string `envconfig:"STARHUB_DATABASE_TIMEZONE" default:"Asia/Shanghai"`
	}

	Redis struct {
		Enable             bool   `envconfig:"STARHUB_SERVER_REDIS_ENABLE" default:"false"`
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
		URL       string `envconfig:"STARHUB_SERVER_GITSERVER_URL"    default:"http://localhost:3000"`
		Type      string `envconfig:"STARHUB_SERVER_GITSERVER_TYPE"    default:"gitea"`
		Host      string `envconfig:"STARHUB_SERVER_GITSERVER_HOST"       default:"http://localhost:3000"`
		SecretKey string `envconfig:"STARHUB_SERVER_GITSERVER_SECRET_KEY" default:"619c849c49e03754454ccd4cda79a209ce0b30b3"`
		Username  string `envconfig:"STARHUB_SERVER_GITSERVER_USERNAME" default:"root"`
		Password  string `envconfig:"STARHUB_SERVER_GITSERVER_PASSWORD" default:"password123"`
	}

	Frontend struct {
		URL string `envconfig:"STARHUB_SERVER_FRONTEND_URL" default:"https://portal-stg.opencsg.com"`
	}

	S3 struct {
		Enable          bool   `envconfig:"STARHUB_SERVER_S3_ENABLE" default:"true"`
		AccessKeyID     string `envconfig:"STARHUB_SERVER_S3_ACCESS_KEY_ID"`
		AccessKeySecret string `envconfig:"STARHUB_SERVER_S3_ACCESS_KEY_SECRET"`
		Region          string `envconfig:"STARHUB_SERVER_S3_REGION"`
		Endpoint        string `envconfig:"STARHUB_SERVER_S3_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
		Bucket          string `envconfig:"STARHUB_SERVER_S3_BUCKET" default:"opencsg-test"`
	}

	SensitiveCheck struct {
		Enable          bool   `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE" default:"false"`
		AccessKeyID     string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID"`
		AccessKeySecret string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET"`
		Region          string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_REGION"`
		Endpoint        string `envconfig:"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT" default:"oss-cn-beijing.aliyuncs.com"`
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
