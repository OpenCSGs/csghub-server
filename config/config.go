package config

type Config struct {
	InstanceID    string `envconfig:"STARHUB_API_INSTANCE_ID"`
	EnableSwagger bool   `envconfig:"STARHUB_API_ENABLE_SWAGGER" default:"false"`

	APIServer struct {
		Port         int    `envconfig:"STARHUB_API_SERVER_PORT" default:"8080"`
		ExternalHost string `envconfig:"STARHUB_API_SERVER_EXTERNAL_HOST" default:"http://localhost"`
	}

	DocsHost string `envconfig:"STARHUB_API_SERVER_DOCS_HOST" default:"http://localhost:6636"`

	Database struct {
		Driver   string `envconfig:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN      string `envconfig:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_api?sslmode=disable"`
		TimeZone string `envconfig:"STARHUB_DATABASE_TIMEZONE" default:"Asia/Shanghai"`
	}

	Redis struct {
		Endpoint           string `envconfig:"STARHUB_API_REDIS_ENDPOINT"              default:"localhost:6379"`
		MaxRetries         int    `envconfig:"STARHUB_API_REDIS_MAX_RETRIES"           default:"3"`
		MinIdleConnections int    `envconfig:"STARHUB_API_REDIS_MIN_IDLE_CONNECTIONS"  default:"0"`
		User               string `envconfig:"STARHUB_API_REDIS_USER"`
		Password           string `envconfig:"STARHUB_API_REDIS_PASSWORD"`
		SentinelMode       bool   `envconfig:"STARHUB_API_REDIS_USE_SENTINEL"          default:"false"`
		SentinelMaster     string `envconfig:"STARHUB_API_REDIS_SENTINEL_MASTER"`
		SentinelEndpoint   string `envconfig:"STARHUB_API_REDIS_SENTINEL_ENDPOINT"`
	}

	GitServer struct {
		Host      string `envconfig:"STARHUB_API_GITSERVER_HOST"`
		SecretKey string `envconfig:"STARHUB_API_GITSERVER_SECRET_KEY"`
	}
}
