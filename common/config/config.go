package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mcuadros/go-defaults"
	"github.com/naoina/toml"
	"github.com/sethvargo/go-envconfig"
)

var configFile = ""

// Default model values of ModelConfig, kept in sync with its envconfig
// default tags: the prompt-injection provider compares against them to
// signal an inherited, unconfigured model at startup.
const (
	DefaultGuardModel    = "Qwen/Qwen3Guard-Gen-0.6B"
	DefaultGuardEndpoint = "https://ai.space.opencsg.com"
)

// ModelConfig is the generic per-model configuration shared by every
// LLM-based sensitive check (sensitive-word guard, prompt injection): a model
// name, its endpoint, and custom HTTP headers. The defaults document a
// working sensitive-word setup and keep existing guard deployments
// compatible; each check should still point at its own dedicated model.
type ModelConfig struct {
	// Model is the model identifier sent in the LLM request body (e.g., "Qwen/Qwen3Guard-Gen-0.6B").
	Model string `env:"MODEL" default:"Qwen/Qwen3Guard-Gen-0.6B"`
	// Endpoint is the full URL of the LLM inference service (e.g., "https://ai.space.opencsg.com/v1/chat/completions").
	Endpoint string `env:"ENDPOINT" default:"https://ai.space.opencsg.com"`
	// Headers are custom HTTP headers sent with each request to this model.
	// Common headers:
	//   - "Authorization": API authentication, e.g., "Bearer <token>"
	//   - "Host": virtual host for routing (e.g., "s-qwen-qwen3guard-gen-0-6b-c0v.spaces.a800.external")
	//   - "X-Custom-Header": any other custom header required by the upstream service
	// Note: "Host" is set via req.Host (not req.Header) to comply with Go's HTTP client behavior.
	Headers map[string]string `env:"HEADERS" default:""`
}

// LLMUnsafeRule is a single guard LLM sensitive rule. Level is the risk level
// to match (case-insensitive, e.g. "Unsafe" or "Controversial"). Category is
// optional: when empty, every category of that level matches; otherwise the
// response category labels must contain it.
type LLMUnsafeRule struct {
	Level    string `json:"level" toml:"level"`
	Category string `json:"category" toml:"category"`
}

// LLMUnsafeRules is a list of LLMUnsafeRule entries; any matching rule marks
// the content sensitive.
type LLMUnsafeRules []LLMUnsafeRule

// UnmarshalJSON lets envconfig load the rules from a JSON array env var and
// keeps entries trimmed.
func (r *LLMUnsafeRules) UnmarshalJSON(data []byte) error {
	var rules []LLMUnsafeRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}
	for i := range rules {
		rules[i].Level = strings.TrimSpace(rules[i].Level)
		rules[i].Category = strings.TrimSpace(rules[i].Category)
	}
	*r = rules
	return nil
}

// FlowRedirect registers one callback target allowed for the legacy "flows"
// signin branch.
type FlowRedirect struct {
	Name        string `json:"name" toml:"name"`
	CallbackURL string `json:"callback_url" toml:"callback_url"`
}

// FlowRedirects is the allowlist of signin callback targets; signin redirects
// are issued only to these registered targets, never to client-provided URLs.
type FlowRedirects []FlowRedirect

// UnmarshalJSON lets envconfig load the redirects from a JSON array env var;
// empty input (unset env or TOML key) decodes to an empty allowlist.
func (r *FlowRedirects) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == `""` {
		*r = nil
		return nil
	}
	var redirects []FlowRedirect
	if err := json.Unmarshal(data, &redirects); err != nil {
		return err
	}
	*r = redirects
	return nil
}

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
	NeedEmailVerify          bool              `env:"STARHUB_SERVER_NEED_EMAIL_VERIFY" default:"true"`
	// TimeZone is the application-wide timezone used for formatting
	// timestamps (e.g. Loki log timestamps).
	TimeZone            string `env:"STARHUB_SERVER_TIMEZONE" default:"Asia/Shanghai"`
	PluginPath          string `env:"STARHUB_SERVER_PLUGIN_PATH" default:"/starhub-bin/plugins"`
	StreamMaxAgeSeconds int    `env:"OPENCSG_DEFAULT_STREAM_MAX_AGE_SECONDS" default:"1209600"` // 14 days

	PostHog struct {
		Enabled      bool   `env:"STARHUB_SERVER_POSTHOG_ENABLED" default:"false"`
		ProjectToken string `env:"STARHUB_SERVER_POSTHOG_PROJECT_TOKEN" default:""`
		APIHost      string `env:"STARHUB_SERVER_POSTHOG_API_HOST" default:""`
		Environment  string `env:"STARHUB_SERVER_POSTHOG_ENVIRONMENT" default:"development"`
	}

	// Organization controls optional hierarchy-backed organization membership.
	Organization struct {
		// EnableUnit requests hierarchy mode in EE/SaaS; CE always uses single-level organizations.
		// Consumers must use IsHierarchicalOrganization to resolve the effective mode.
		EnableUnit bool `env:"STARHUB_SERVER_ORGANIZATION_ENABLE_UNIT" default:"false"`
	}

	// ReBAC controls the embedded OpenFGA authorization service.
	Rebac struct {
		// OpenFGAListObjectMaxResult limits the number of objects returned by OpenFGA ListObjects.
		OpenFGAListObjectMaxResult int `env:"STARHUB_SERVER_REBAC_OPENFGA_LIST_OBJECT_MAX_RESULT" default:"1000"`
	}

	// CORS controls the browser cross-origin policy shared by the API,
	// AIGateway, and rproxy routers. It replaces the previous
	// AllowAllOrigins+AllowCredentials combination (CWE-942, issue #1430).
	CORS struct {
		// AllowedOrigins is the explicit whitelist of browser origins allowed
		// to read responses cross-origin. Entries are exact origins such as
		// "https://hub.opencsg.com" or per-label subdomain wildcards such as
		// "https://*.opencsg.com". Setting at least one valid entry makes this
		// list the whole whitelist and disables derivation. When it is empty
		// the middleware falls back to the deployment's own addresses: the
		// origins of Frontend.URL and APIServer.PublicDomain (exact), and a
		// per-label wildcard for both schemes over Space.PublicRootDomain.
		// Requests without an Origin header (curl, same-origin browser pages)
		// are always unaffected.
		AllowedOrigins []string `env:"STARHUB_SERVER_CORS_ALLOWED_ORIGINS" default:"[]"`
	}

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
		RateBucketCapacity int `env:"STARHUB_SERVER_MIRROR_RATE_BUCKET_CAPACITY" default:"1"`
		MaxRetryCount      int `env:"STARHUB_SERVER_MIRROR_MAX_RETRY_COUNT" default:"3"`
		// Metrics controls ephemeral runtime reporting shared by API, repo, and LFS processes.
		Metrics struct {
			Enabled bool   `env:"STARHUB_SERVER_MIRROR_METRICS_ENABLED" default:"true"`
			Scope   string `env:"STARHUB_SERVER_MIRROR_METRICS_SCOPE" default:"default"`
		}
	}

	DocsHost string `env:"STARHUB_SERVER_SERVER_DOCS_HOST" default:"http://localhost:6636"`

	Database struct {
		Driver              string `env:"STARHUB_DATABASE_DRIVER" default:"pg"`
		DSN                 string `env:"STARHUB_DATABASE_DSN" default:"postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable"`
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
		URL string `env:"STARHUB_SERVER_FRONTEND_URL"`
	}

	S3 struct {
		AccessKeyID     string `env:"STARHUB_SERVER_S3_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"STARHUB_SERVER_S3_ACCESS_KEY_SECRET"`
		Region          string `env:"STARHUB_SERVER_S3_REGION"`
		Endpoint        string `env:"STARHUB_SERVER_S3_ENDPOINT" default:"localhost:9000"`
		// EnableDualstack controls AWS S3 dual-stack (IPv4+IPv6) endpoints. When true,
		// an amazonaws.com Endpoint is normalized to its s3.dualstack form at config
		// load so the host minio signs matches the host the storage gateway forwards to.
		EnableDualstack bool `env:"STARHUB_SERVER_S3_ENABLE_DUALSTACK" default:"true"`
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
		// OSSBucket is the Aliyun OSS bucket used for temporary image uploads
		// when checking images from private repos (where no public URL is
		// available). The bucket must be in the same Aliyun account as
		// AccessKeyID/AccessKeySecret so that Aliyun Green can read it directly.
		// Required only when ImageCheckEnable is true and private repos need
		// image moderation.
		OSSBucket string `env:"STARHUB_SERVER_SENSITIVE_CHECK_OSS_BUCKET" default:"sensitive-check"`

		DictDir string `env:"STARHUB_SERVER_SENSITIVE_CHECK_DICT_DIR" default:"/starhub-bin/vocabulary"`
		// CheckChain selects and orders the sensitive-check providers. Built-in
		// providers: ac_automaton, mutable_ac_automaton, aliyun_green. EE/SaaS
		// additionally registers the LLM providers guard_llm (sensitive-word
		// guard) and prompt_injection (issue #1581); both need their Enable
		// flag and model config set. The chain short-circuits on the first
		// sensitive verdict, so ordering matters: a provider earlier in the
		// chain that flags sensitive suppresses the later ones.
		//
		// Budget: the caller's prompt-check RPC timeout (2s in aigateway)
		// covers the WHOLE chain, so the sum of the LLM providers' TimeoutMS
		// must stay below it — guard_llm (3s) and prompt_injection (1.5s)
		// together exceed the budget and the gateway fails that prompt check
		// open (one warn log, no block). Enable both LLM providers on the
		// prompt path only together with a raised caller budget.
		CheckChain          []string `env:"STARHUB_SERVER_SENSITIVE_CHECK_CHECK_CHAIN" default:"[ac_automaton,mutable_ac_automaton,aliyun_green]"`
		StreamCheckMode     string   `env:"STARHUB_SERVER_SENSITIVE_CHECK_STREAM_CHECK_MODE" default:"async"` // sync | async
		AsyncBufferMaxChars int      `env:"STARHUB_SERVER_SENSITIVE_CHECK_ASYNC_BUFFER_MAX_CHARS" default:"50"`
		// aliyun green max content length: 2000 | qwen guard max content length: 7000
		MaxContentLength int `env:"STARHUB_SERVER_SENSITIVE_CHECK_MAX_CONTENT_LENGTH" default:"2000"`
		// PromptMaxChunks caps how many moderation calls a single LLM prompt is
		// worth, estimated as ceil(prompt length / MaxContentLength). A prompt
		// within one call is checked synchronously as before; a prompt worth
		// more than one call but within this limit is checked asynchronously
		// (the request proceeds without waiting for the verdict); a prompt
		// exceeding this limit skips prompt moderation entirely and only logs.
		// Zero or negative keeps the legacy behavior: every prompt is checked
		// synchronously regardless of size.
		PromptMaxChunks int `env:"STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_MAX_CHUNKS" default:"3"`
		// ImageCheckEnable controls whether image files are sent to the remote
		// moderation service for content safety checking. Disabled by default
		// because image moderation incurs extra API costs.
		ImageCheckEnable bool `env:"STARHUB_SERVER_SENSITIVE_CHECK_IMAGE_ENABLE" default:"false"`
		// ImageCheckScenario is the Aliyun Green scenario used for image moderation.
		// Defaults to "baselineCheck" (ScenarioImageBaseLineCheck).
		ImageCheckScenario string `env:"STARHUB_SERVER_SENSITIVE_CHECK_IMAGE_SCENARIO" default:"baselineCheck"`
		// ImageCheckMaxRefsPerComment caps synchronous image checks per comment.
		ImageCheckMaxRefsPerComment int `env:"STARHUB_SERVER_SENSITIVE_CHECK_IMAGE_MAX_REFS" default:"5"`
		// CommentMediaMaxRefsPerComment caps all image/audio/video references before
		// any external moderation service is called.
		CommentMediaMaxRefsPerComment int `env:"STARHUB_SERVER_SENSITIVE_CHECK_COMMENT_MEDIA_MAX_REFS" default:"5"`

		// MediaModerationEnable controls asynchronous audio/video moderation for
		// discussion comments. When enabled, a comment referencing pending
		// audio/video is created in a pending state (visible only to its author)
		// and a temporal workflow polls the Aliyun result. It is effective only
		// when SensitiveCheck.Enable is also true. Disabled by default.
		MediaModerationEnable bool `env:"STARHUB_SERVER_SENSITIVE_CHECK_MEDIA_MODERATION_ENABLE" default:"false"`
		// MediaModerationMaxRefsPerComment caps how many audio/video items a
		// single comment may reference.
		MediaModerationMaxRefsPerComment int `env:"STARHUB_SERVER_SENSITIVE_CHECK_MEDIA_MODERATION_MAX_REFS" default:"3"`
		// MediaModerationPollInterval is how often the poll workflow queries the
		// Aliyun result for each pending media task.
		MediaModerationPollInterval time.Duration `env:"STARHUB_SERVER_SENSITIVE_CHECK_MEDIA_MODERATION_POLL_INTERVAL" default:"30s"`
		// MediaModerationWorkflowTimeout bounds how long the poll workflow runs
		// before it finalizes (and deletes the comment if media is still
		// non-terminal).
		MediaModerationWorkflowTimeout time.Duration `env:"STARHUB_SERVER_SENSITIVE_CHECK_MEDIA_MODERATION_WORKFLOW_TIMEOUT" default:"30m"`

		LLM struct {
			Enable       bool    `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_ENABLE" default:"false"`
			TimeoutMS    int     `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_TIMEOUT_MS" default:"3000"`
			MaxTokens    int     `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_MAX_TOKENS" default:"128"`
			Temperature  float64 `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_TEMPERATURE" default:"0"`
			ResponseMode string  `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_RESPONSE_MODE" default:"json_or_text"`
			SafetyRegex  string  `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_SAFETY_REGEX" default:"Safety:\\s*(Safe|Unsafe|Controversial)"`
			// UnsafeRules defines when a guard LLM verdict marks content as
			// sensitive. The content is sensitive when ANY rule matches: the
			// response risk level equals the rule level (case-insensitive) and
			// either the rule category is empty (any category of that level
			// matches) or the response categories include the rule category.
			// Rules are OR-combined. An empty list disables sensitive blocking
			// for guard LLM responses. Configured as a JSON array, e.g.
			// [{"level":"Unsafe"},{"level":"Controversial","category":"Politically Sensitive Topics"}]
			UnsafeRules LLMUnsafeRules `env:"STARHUB_SERVER_SENSITIVE_CHECK_LLM_UNSAFE_RULES,default=[{\"level\":\"Unsafe\"},{\"level\":\"Controversial\",\"category\":\"Politically Sensitive Topics\"}]"`
			// Guard is the model config for non-stream sensitive checks.
			// Each must have a Model and Endpoint. Headers (including Authorization) are per-model.
			Guard ModelConfig `env:",prefix=STARHUB_SERVER_SENSITIVE_CHECK_LLM_GUARD_"`
			// GuardStream is the model config for stream sensitive checks.
			GuardStream ModelConfig `env:",prefix=STARHUB_SERVER_SENSITIVE_CHECK_LLM_GUARD_STREAM_"`
		}

		// PromptInjection configures the LLM-based prompt-injection detection
		// provider. It is a member of CheckChain (add "prompt_injection" to
		// the chain to activate it), mirroring the guard_llm provider: its own
		// dedicated decision-model endpoint speaking the SystemOne protocol —
		// never the sensitive-word guard model above. In this first version
		// only prompt checks carry logic; the image and response slots of the
		// SensitiveChecker interface are no-ops, ready for later phases.
		PromptInjection struct {
			// Enable turns the provider on. Effective only when
			// SensitiveCheck.Enable is also true AND "prompt_injection" is a
			// member of CheckChain (same double gate as guard_llm).
			Enable bool `env:"STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_INJECTION_ENABLE" default:"false"`
			// Mode controls enforcement: "block" marks the prompt sensitive
			// when the risk score crosses the threshold; "detect" only records
			// the verdict (shadow mode, never blocks).
			Mode string `env:"STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_INJECTION_MODE" default:"block"`
			// RiskThreshold is the injection risk score (0-1) at or above which
			// the prompt counts as an injection attempt. The default is
			// calibrated for convaiinnovations/laya by the issue #1581
			// evaluation (86% attack detection, 0% benign false positives).
			// For the more conservative jev-1.13 scoring, lower it to 0.4-0.5
			// (same 0% false positives at 0.3-0.5 in the evaluation).
			RiskThreshold float64 `env:"STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_INJECTION_RISK_THRESHOLD" default:"0.6"`
			// TimeoutMS bounds the decision-model call. It must stay below the
			// 2s RPC timeout aigateway applies to prompt checks, and it counts
			// against the whole-chain budget (see CheckChain).
			TimeoutMS int `env:"STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_INJECTION_TIMEOUT_MS" default:"1500"`
			// Model is the dedicated decision model (SystemOne protocol),
			// reusing the generic ModelConfig: STARHUB_SERVER_SENSITIVE_CHECK_
			// PROMPT_INJECTION_MODEL / _ENDPOINT / _HEADERS. When enabling the
			// provider, override all three (the inherited defaults document
			// the sensitive-word guard setup); an unconfigured injection
			// simply fails open — the wrong endpoint/auth 401s or 404s and
			// every check degrades to skipped_api_error.
			//	convaiinnovations/laya: ~0.4s mean latency; RiskThreshold 0.6
			//	jev-1.13: ~1.5s mean latency, conservative scores; 0.4-0.5
			Model ModelConfig `env:",prefix=STARHUB_SERVER_SENSITIVE_CHECK_PROMPT_INJECTION_"`
		}
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
		PublicRootDomain string `env:"STARHUB_SERVER_PUBLIC_ROOT_DOMAIN" default:""`
		DockerRegBase    string `env:"STARHUB_SERVER_DOCKER_REG_BASE" default:"registry.cn-beijing.aliyuncs.com/opencsg_public/"`
		ImagePullSecret  string `env:"STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET" default:"opencsg-pull-secret"`
		// reverse proxy listening port
		RProxyServerPort int `env:"STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT" default:"8083"`
		// secret key for session encryption
		SessionSecretKey          string `env:"STARHUB_SERVER_SPACE_SESSION_SECRET_KEY" default:"secret"`
		DeployTimeoutInMin        int    `env:"STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES" default:"30"`
		BuildTimeoutInMin         int    `env:"STARHUB_SERVER_SPACE_BUILD_TIMEOUT_IN_MINUTES" default:"30"`
		ReadinessDelaySeconds     int    `env:"STARHUB_SERVER_READINESS_DELAY_SECONDS" default:"120"`
		ReadinessPeriodSeconds    int    `env:"STARHUB_SERVER_READINESS_PERIOD_SECONDS" default:"10"`
		ReadinessFailureThreshold int    `env:"STARHUB_SERVER_READINESS_FAILURE_THRESHOLD" default:"3"`
		PYPIIndexURL              string `env:"STARHUB_SERVER_SPACE_PYPI_INDEX_URL" default:""`
		InformerSyncPeriodInMin   int    `env:"STARHUB_SERVER_SPACE_INFORMER_SYNC_PERIOD_IN_MINUTES" default:"2"`
		StatusCheckInterval       int    `env:"STARHUB_SERVER_SPACE_STATUS_CHECK_INTERVAL" default:"10"` // 10 seconds
		SandboxPVCName            string `env:"STARHUB_SERVER_SANDBOX_PVC_NAME" default:"sandbox-storage"`
		SandboxRuntimeClass       string `env:"STARHUB_SERVER_SANDBOX_RUNTIME_CLASS" default:"gvisor"`
		// SandboxFreeResourceMaxIdleTimeoutMin: the idle-reclaim timeout (in minutes) applied to
		// sandboxes created on free resources (price == 0). It acts as BOTH the default (when the
		// caller does not pass a timeout) and the upper bound (a larger caller-provided timeout is
		// clamped down to this value), so free resources cannot be held indefinitely. Defaults to 10.
		SandboxFreeResourceMaxIdleTimeoutMin int `env:"STARHUB_SERVER_SANDBOX_FREE_RESOURCE_MAX_IDLE_TIMEOUT_MIN" default:"10"`
		// SandboxIdleSweepIntervalSec is the idle scan period. The scan reads the informer cache, zero API calls.
		SandboxIdleSweepIntervalSec int `env:"STARHUB_SERVER_SANDBOX_IDLE_SWEEP_INTERVAL_SECONDS" default:"60"`
		// SandboxIdleStartupGraceMin: for how many minutes after Runner startup only record activity and do not
		// suspend, to avoid an instant batch suspend right after a restart.
		SandboxIdleStartupGraceMin int `env:"STARHUB_SERVER_SANDBOX_IDLE_STARTUP_GRACE_MINUTES" default:"5"`
	}

	Model struct {
		DeployTimeoutInMin        int    `env:"STARHUB_SERVER_MODEL_DEPLOY_TIMEOUT_IN_MINUTES" default:"60"`
		DownloadEndpoint          string `env:"STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT" default:"https://hub.opencsg.com"`
		DockerRegBase             string `env:"STARHUB_SERVER_MODEL_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com"`
		NimDockerSecretName       string `env:"STARHUB_SERVER_MODEL_NIM_DOCKER_SECRET_NAME" default:"ngc-secret"`
		NimNGCSecretName          string `env:"STARHUB_SERVER_MODEL_NIM_NGC_SECRET_NAME" default:"nvidia-nim-secrets"`
		MinContextForEstimation   int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_ESTIMATION" default:"2048"`
		MinContextForFinetune     int    `env:"STARHUB_SERVER_MODEL_MIN_CONTEXT_FOR_FINETUNE" default:"512"`
		DeployStatusCheckInterval int    `env:"STARHUB_SERVER_MODEL_DEPLOY_STATUS_CHECK_INTERVAL" default:"10"` // 10 seconds
		// PDRecommendParamsThreshold is the minimum model parameter count (in billions)
		// for which PD disaggregation recommendations are generated. Models with params
		// below this threshold are skipped. Default is 100B.
		PDRecommendParamsThreshold float32 `env:"STARHUB_SERVER_MODEL_PD_RECOMMEND_PARAMS_THRESHOLD" default:"100"`
		// AutoSpeculativeDecoding enables automatic MTP speculative decoding for
		// model deployments whose checkpoint embeds MTP weights, when the user did
		// not select a spec-decode method. Default is false.
		AutoSpeculativeDecoding bool `env:"STARHUB_SERVER_MODEL_AUTO_SPECULATIVE_DECODING" default:"false"`
	}

	Search struct {
		RepoSearchCacheTTL           int `env:"STARHUB_SERVER_REPO_SEARCH_CACHE_TTL" default:"300"`           // 5 min
		RepositoryAccessListCacheTTL int `env:"STARHUB_SERVER_REPOSITORY_ACCESS_LIST_CACHE_TTL" default:"60"` // 1 min
		RepoSearchLimit              int `env:"STARHUB_SERVER_REPO_SEARCH_LIMIT" default:"2000"`
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
		InvitationQuota  int    `env:"STARHUB_SERVER_CASDOOR_INVITATION_QUOTA" default:"50"`
	}

	Paraview struct {
		Endpoint     string `env:"STARHUB_SERVER_PARAVIEW_ENDPOINT" default:"https://iam-c.paraview.cn"`
		ClientID     string `env:"STARHUB_SERVER_PARAVIEW_CLIENT_ID" default:""`
		ClientSecret string `env:"STARHUB_SERVER_PARAVIEW_CLIENT_SECRET" default:""`
		RedirectURI  string `env:"STARHUB_SERVER_PARAVIEW_REDIRECT_URI" default:"http://127.0.0.1:8080/api/v1/callback/paraview"`
		ApiKey       string `env:"STARHUB_SERVER_PARAVIEW_API_KEY" default:""`
		ApiSecret    string `env:"STARHUB_SERVER_PARAVIEW_API_SECRET" default:""`
	}

	Portico struct {
		Endpoint     string `env:"STARHUB_SERVER_PORTICO_ENDPOINT" default:""`
		ClientID     string `env:"STARHUB_SERVER_PORTICO_CLIENT_ID" default:""`
		ClientSecret string `env:"STARHUB_SERVER_PORTICO_CLIENT_SECRET" default:""`
		RedirectURI  string `env:"STARHUB_SERVER_PORTICO_REDIRECT_URI" default:"http://127.0.0.1:8080/api/v1/callback/portico"`
		Scopes       string `env:"STARHUB_SERVER_PORTICO_SCOPES" default:"openid profile email phone"`
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
		SubscriptionCronExpression   string `env:"OPENCSG_ACCOUNTING_SUBSCRIPTION_CRON_EXPRESSION" default:"*/10 * * * *"`
		ExpiredPresentCronExpression string `env:"OPENCSG_ACCOUNTING_EXPIRED_PRESENT_CRON_EXPRESSION" default:"0 0 * * *"`
		// AutoExpirePresent bounds the grant window of registration-award
		// presents (activity CSHHubNewUser) that are force expired
		// AutoExpirePresentValidDays days after the present is granted
		// (account_presents.created_at). Empty dates or a non-positive day
		// count disables the auto-expire scan.
		AutoExpirePresentStartDate string `env:"OPENCSG_ACCOUNTING_AUTO_EXPIRE_PRESENT_START_DATE" default:"2026-10-14"`
		AutoExpirePresentEndDate   string `env:"OPENCSG_ACCOUNTING_AUTO_EXPIRE_PRESENT_END_DATE" default:"2026-10-30"`
		AutoExpirePresentValidDays int    `env:"OPENCSG_ACCOUNTING_AUTO_EXPIRE_PRESENT_VALID_DAYS" default:"30"`
		// AutoExpirePresentBatchSize bounds one auto-expire activity page. It
		// keeps the activity, its memory and its retry blast radius small when
		// the grant window contains a large number of presents.
		AutoExpirePresentBatchSize  int    `env:"OPENCSG_ACCOUNTING_AUTO_EXPIRE_PRESENT_BATCH_SIZE" default:"500"`
		ThresholdOfStopDeploy       int    `env:"OPENCSG_ACCOUNTING_THRESHOLD_OF_STOP_DEPLOY" default:"0"`
		ThresholdOfStopLLMInference int    `env:"OPENCSG_ACCOUNTING_THRESHOLD_OF_STOP_LLM_INFERENCE" default:"0"`
		LLMBalanceCheckCacheTTL     int    `env:"OPENCSG_ACCOUNTING_LLM_BALANCE_CHECK_CACHE_TTL" default:"120"`
		RetryLimit                  int    `env:"OPENCSG_ACCOUNTING_RETRY_LIMIT" default:"1"`
		MaxExportPages              int    `env:"OPENCSG_ACCOUNTING_MAX_EXPORT_PAGES" default:"1000"`
		AllowedSyncDeductScenes     string `env:"OPENCSG_ACCOUNTING_ALLOWED_SYNC_DEDUCT_SCENES" default:"30"`
		AccessQuotaTTLInHours       int    `env:"OPENCSG_ACCOUNTING_ACCESS_QUOTA_TTL_IN_HOURS" default:"8"`
	}

	User struct {
		// EnableAlipayIdentity initializes the SaaS identity client; disable only to develop without Alipay credentials.
		EnableAlipayIdentity           bool   `env:"OPENCSG_USER_SERVER_ENABLE_ALIPAY_IDENTITY" default:"true"`
		Host                           string `env:"OPENCSG_USER_SERVER_HOST" default:"http://localhost"`
		Port                           int    `env:"OPENCSG_USER_SERVER_PORT" default:"8088"`
		SigninSuccessRedirectURL       string `env:"OPENCSG_USER_SERVER_SIGNIN_SUCCESS_REDIRECT_URL" default:"http://localhost:3000/server/callback"`
		CodeSoulerVScodeRedirectURL    string `env:"OPENCSG_USER_SERVER_CODESOULER_VSCODE_REDIRECT_URL" default:"http://127.0.0.1:37678/callback"`
		CodeSoulerJetBrainsRedirectURL string `env:"OPENCSG_USER_SERVER_CODESOULER_JETBRAINS_REDIRECT_URL" default:"http://127.0.0.1:37679/callback"`
		LiteRedirectURL                string `env:"OPENCSG_USER_SERVER_LITE_REDIRECT_URL" default:"http://127.0.0.1:11437/api/cloud/auth/callback"`
		// FlowRedirects is the allowlist of legacy "flows" signin callback
		// targets as a JSON array, e.g.
		// [{"name":"langflow","callback_url":"https://langflow.example.com/api/v1/callback/casdoor"}].
		// The signin callback never redirects to URLs outside this list.
		FlowRedirects FlowRedirects `env:"OPENCSG_USER_SERVER_FLOW_REDIRECTS" default:"[]"`
		// NamespaceLookupMaxUUIDs limits the number of namespace UUIDs accepted by
		// the batch namespace lookup endpoint.
		NamespaceLookupMaxUUIDs int `env:"OPENCSG_USER_SERVER_NAMESPACE_LOOKUP_MAX_UUIDS" default:"100"`
	}

	Credential struct {
		// Default decodes to "opencsg-credential-dev-key-00001" for local/dev use; override in production.
		MasterKeyBase64                  string `env:"OPENCSG_CREDENTIAL_MASTER_KEY_BASE64" default:"b3BlbmNzZy1jcmVkZW50aWFsLWRldi1rZXktMDAwMDE="`
		SecretBackend                    string `env:"OPENCSG_CREDENTIAL_SECRET_BACKEND" default:"postgres_encrypted"`
		RuntimeSessionSigningKey         string `env:"OPENCSG_CREDENTIAL_RUNTIME_SESSION_SIGNING_KEY" default:"credential-runtime-signing-key"`
		RuntimeSessionMaxDurationSeconds int    `env:"OPENCSG_CREDENTIAL_RUNTIME_SESSION_MAX_DURATION_SECONDS" default:"3600"`
		VaultAddress                     string `env:"OPENCSG_CREDENTIAL_VAULT_ADDRESS" default:""`
		VaultToken                       string `env:"OPENCSG_CREDENTIAL_VAULT_TOKEN" default:""`
		VaultNamespace                   string `env:"OPENCSG_CREDENTIAL_VAULT_NAMESPACE" default:""`
		VaultKVDefaultMount              string `env:"OPENCSG_CREDENTIAL_VAULT_KV_DEFAULT_MOUNT" default:"secret"`
		VaultTimeoutSeconds              int    `env:"OPENCSG_CREDENTIAL_VAULT_TIMEOUT_SECONDS" default:"5"`
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

	Academy struct {
		Host string `env:"OPENCSG_ACADEMY_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_ACADEMY_SERVER_PORT" default:"8000"`
	}

	Supplydemand struct {
		Host string `env:"OPENCSG_SUPPLYDEMAND_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_SUPPLYDEMAND_SERVER_PORT" default:"8000"`
	}

	// for csghub agents
	CSGBot struct {
		Host string `env:"OPENCSG_CSGBOT_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_CSGBOT_SERVER_PORT" default:"8070"`
	}

	LLMWiki struct {
		Host string `env:"OPENCSG_LLMWIKI_SERVER_HOST" default:"http://127.0.0.1"`
		Port int    `env:"OPENCSG_LLMWIKI_SERVER_PORT" default:"8100"`
	}

	Moderation struct {
		Host                     string `env:"OPENCSG_MODERATION_SERVER_HOST" default:"http://localhost"`
		Port                     int    `env:"OPENCSG_MODERATION_SERVER_PORT" default:"8089"`
		RepoFileCheckConcurrency int    `env:"OPENCSG_MODERATION_SERVER_REPO_FILE_CHECK_CONCURRENCY" default:"10"`
		SecurityScan             SecurityScanConfig
	}

	Memory MemoryConfig

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
		MCPInspectCronExpression                 string `env:"STARHUB_SERVER_CRON_JOB_MCP_INSPECT_CRON_EXPRESSION" default:"*/10 * * * *"`
		AIGatewayAsyncGenerationCronExpression   string `env:"STARHUB_SERVER_CRON_JOB_AIGATEWAY_ASYNC_GENERATION_CRON_EXPRESSION" default:"*/1 * * * *"`
		AIGatewayMetricsCollectorCronExpression  string `env:"STARHUB_SERVER_CRON_JOB_AIGATEWAY_METRICS_COLLECTOR_CRON_EXPRESSION" default:"* * * * *"` // every minute
		SyncLLMLogsToDatasetCronExpression       string `env:"STARHUB_SERVER_SYNC_LLMLOGS_TO_DATASET_CRON_EXPRESSION" default:"0 1 * * *"`
		StatementDailySummaryCronExpression      string `env:"STARHUB_SERVER_CRON_JOB_STATEMENT_DAILY_SUMMARY_CRON_EXPRESSION" default:"17 2 * * *"`    // 02:17 daily
		TokenReportDailySummaryCronExpression    string `env:"STARHUB_SERVER_CRON_JOB_TOKEN_REPORT_DAILY_SUMMARY_CRON_EXPRESSION" default:"23 3 * * *"` // 03:23 daily, after statement daily summary
		HistoryArchiveCronExpression             string `env:"STARHUB_SERVER_CRON_JOB_HISTORY_ARCHIVE_CRON_EXPRESSION" default:"30 2 * * *"`            // 02:30 daily (Asia/Shanghai, applied via ScheduleSpec.TimeZoneName)
		UpstreamSyncReconcileCronExpression      string `env:"STARHUB_SERVER_CRON_JOB_UPSTREAM_SYNC_RECONCILE_CRON_EXPRESSION" default:"*/10 * * * *"`  // every 10 minutes
		RepoDescriptionReconcileCronExpression   string `env:"STARHUB_SERVER_CRON_JOB_REPO_DESCRIPTION_RECONCILE_CRON_EXPRESSION" default:"0 18 * * *"` // 02:00 daily (UTC+8)
	}

	HistoryArchive struct {
		Enable           bool `env:"STARHUB_SERVER_HISTORY_ARCHIVE_ENABLE" default:"false"`
		BatchSize        int  `env:"STARHUB_SERVER_HISTORY_ARCHIVE_BATCH_SIZE" default:"50000"`
		MaxBatchesPerRun int  `env:"STARHUB_SERVER_HISTORY_ARCHIVE_MAX_BATCHES_PER_RUN" default:"200"`
		MaxRunMinutes    int  `env:"STARHUB_SERVER_HISTORY_ARCHIVE_MAX_RUN_MINUTES" default:"300"`
	}

	DeployReconcile struct {
		CronExpression        string `env:"STARHUB_SERVER_DEPLOY_RECONCILE_CRON_EXPRESSION" default:"*/10 * * * *"`
		StuckTimeoutMin       int    `env:"STARHUB_SERVER_DEPLOY_RECONCILE_STUCK_TIMEOUT_MIN" default:"15"`
		RunningReconcileHour  int    `env:"STARHUB_SERVER_DEPLOY_RECONCILE_RUNNING_HOUR" default:"2"`
		UnhealthyReconcileMin int    `env:"STARHUB_SERVER_DEPLOY_RECONCILE_UNHEALTHY_MIN" default:"10"`
	}

	Agent struct {
		AutoHubServiceHost                    string `env:"OPENCSG_AGENT_AUTOHUB_SERVICE_HOST" default:"http://internal.opencsg-stg.com:8190"`
		AgentHubServiceHost                   string `env:"OPENCSG_AGENT_AGENTHUB_SERVICE_HOST" default:""`
		AgentHubServiceToken                  string `env:"OPENCSG_AGENT_AGENTHUB_SERVICE_TOKEN" default:""`
		MCPInspectMaxConcurrency              int    `env:"OPENCSG_AGENT_MCP_INSPECT_MAX_CONCURRENCY" default:"50"`
		ShareSessionTokenValidDay             int    `env:"STARHUB_SERVER_AGENT_SHARE_SESSION_TOKEN_VALIDATE_Day" default:"365"` // 1 year
		BalanceThreshold                      int    `env:"OPENCSG_AGENT_BALANCE_THRESHOLD" default:"0"`
		BalanceCheckCacheTTL                  int    `env:"OPENCSG_AGENT_BALANCE_CHECK_CACHE_TTL" default:"86400"`
		OrchestratorReadinessTimeoutSeconds   int    `env:"OPENCSG_AGENT_ORCHESTRATOR_READINESS_TIMEOUT_SECONDS" default:"1800"`
		OrchestratorGatewayWaitTimeoutSeconds int    `env:"OPENCSG_AGENT_ORCHESTRATOR_GATEWAY_WAIT_TIMEOUT_SECONDS" default:"30"`
		// OrchestratorMaxConcurrentExecutionsPerAllocation caps concurrent
		// executions across sessions on one actor runtime. Temporary static
		// value until load-aware admission is available.
		OrchestratorMaxConcurrentExecutionsPerAllocation int `env:"OPENCSG_AGENT_ORCHESTRATOR_MAX_CONCURRENT_EXECUTIONS_PER_ALLOCATION" default:"4"`
		OrchestratorExecutionReservedTimeoutSeconds      int `env:"OPENCSG_AGENT_ORCHESTRATOR_EXECUTION_RESERVED_TIMEOUT_SECONDS" default:"60"`
		OrchestratorExecutionHeartbeatIntervalSeconds    int `env:"OPENCSG_AGENT_ORCHESTRATOR_EXECUTION_HEARTBEAT_INTERVAL_SECONDS" default:"30"`
		OrchestratorExecutionHeartbeatTimeoutSeconds     int `env:"OPENCSG_AGENT_ORCHESTRATOR_EXECUTION_HEARTBEAT_TIMEOUT_SECONDS" default:"90"`
		OrchestratorExecutionUnknownCapacityHoldSeconds  int `env:"OPENCSG_AGENT_ORCHESTRATOR_EXECUTION_UNKNOWN_CAPACITY_HOLD_SECONDS" default:"3600"`
		OrchestratorResumeWaitTimeoutSeconds             int `env:"OPENCSG_AGENT_ORCHESTRATOR_RESUME_WAIT_TIMEOUT_SECONDS" default:"60"`
		// UseLatestImage replaces the csgclaw runtime profile image tag with :latest;
		// Kubernetes then defaults the sandbox container's imagePullPolicy to Always.
		// Intended for staging environments where images are updated frequently;
		// defaults to false.
		UseLatestImage bool `env:"OPENCSG_AGENT_USE_LATEST_IMAGE" default:"false"`
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
		ScanFileNumLimit                        int    `env:"OPENCSG_DATAVIEWER_SCAN_FILE_NUM_LIMIT" default:"5000"`      // 5K files
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
		MaxHFCommitBodySize    int64  `env:"STARHUB_SERVER_GIT_MAX_HF_COMMIT_BODY_SIZE" default:"268435456"`
		SkipLfsFileValidation  bool   `env:"STARHUB_SERVER_SKIP_LFS_FILE_VALIDATION" default:"false"`
		SignatureSecertKey     string `env:"STARHUB_SERVER_GIT_SIGNATURE_SECRET_KEY" default:"git-secret"`
		MinMultipartSize       int64  `env:"STARHUB_SERVER_GIT_MIN_MULTIPART_SIZE" default:"52428800"`
		LfsExistsCheck         bool   `env:"STARHUB_SERVER_GIT_LFS_EXISTS_CHECK" default:"true"`
		RepoDataMigrateEnable  bool   `env:"STARHUB_SERVER_GIT_REPO_DATA_MIGRATE_ENABLE" default:"false"`
		LimitLfsFileUploadSize bool   `env:"STARHUB_SERVER_GIT_LIMIT_LFS_FILE_UPLOAD_SIZE " default:"true"`
		TreeOperationTimeout   int    `env:"STARHUB_SERVER_GIT_TREE_OPERATION_TIMEOUT" default:"3"`
		FetchTimeout           int    `env:"STARHUB_SERVER_GIT_FETCH_TIMEOUT" default:"240"`
		MaxArchiveSizeMB       int64  `env:"STARHUB_SERVER_GIT_MAX_ARCHIVE_SIZE_MB" default:"50"`
		// Welcome message displayed after git push
		WelcomeMessage string `env:"STARHUB_SERVER_GIT_WELCOME_MESSAGE" default:"Welcome to OpenCSG!"`
	}

	AIGateway struct {
		Port                                   int    `env:"OPENCSG_AIGATEWAY_PORT" default:"8094"`
		AdvertiseAddr                          string `env:"OPENCSG_AIGATEWAY_ADVERTISE_ADDR" default:""`
		PublicAIGatewayURL                     string `env:"OPENCSG_AIGATEWAY_PUBLIC_URL" default:""`
		SensitiveDefaultImg                    string `env:"STARHUB_SERVER_AIGATEWAY_SENSITIVE_DEFAULT_IMG" default:""`
		PresignExpirySeconds                   int    `env:"OPENCSG_AIGATEWAY_PRESIGN_EXPIRY" default:"86400"`
		ResponsesIDSecret                      string `env:"OPENCSG_AIGATEWAY_RESPONSES_ID_SECRET" default:"responses-id-secret"`
		EnableLLMLog                           bool   `env:"OPENCSG_AIGATEWAY_LLMLOG_ENABLE" default:"true"`
		EnableLLMLogCompress                   bool   `env:"OPENCSG_AIGATEWAY_LLMLOG_COMPRESS_ENABLE" default:"false"`
		EnableLLMTrace                         bool   `env:"OPENCSG_AIGATEWAY_LLM_TRACE_ENABLE" default:"true"`
		LLMTraceContentCapture                 string `env:"OPENCSG_AIGATEWAY_LLM_TRACE_CONTENT_CAPTURE" default:"metadata_only"`
		LLMTraceMaxContentLength               int    `env:"OPENCSG_AIGATEWAY_LLM_TRACE_MAX_CONTENT_LENGTH" default:"256"`
		LLMTraceMaxInputUserMessages           int    `env:"OPENCSG_AIGATEWAY_LLM_TRACE_MAX_INPUT_USER_MESSAGES" default:"1"`
		LLMTraceToolDefinitionsMode            string `env:"OPENCSG_AIGATEWAY_LLM_TRACE_TOOL_DEFINITIONS_MODE" default:"name_only"`
		ChatMaxFallbackAttempts                int    `env:"OPENCSG_AIGATEWAY_CHAT_MAX_FALLBACK_ATTEMPTS" default:"2"`
		HealthCheckEnabled                     bool   `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_ENABLED" default:"true"`
		HealthCheckL7APIEnabled                bool   `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_ENABLED" default:"true"`
		HealthCheckL7APIInterval               int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_INTERVAL" default:"60"`
		HealthCheckL7APITimeout                int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_TIMEOUT" default:"15"`
		HealthCheckProbeWorkers                int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_PROBE_WORKERS" default:"32"`
		HealthCheckPersistenceWorkers          int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_PERSISTENCE_WORKERS" default:"4"`
		HealthCheckModalInferenceInterval      int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_INFERENCE_INTERVAL" default:"3600"`
		HealthCheckConsecutiveFailures         int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_CONSECUTIVE_FAILURES" default:"3"`
		HealthCheckLatencyDegradedMs           int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_LATENCY_DEGRADED_MS" default:"10000"`
		HealthCheckMultimodalLatencyDegradedMs int    `env:"OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_LATENCY_DEGRADED_MS" default:"120000"`
		CircuitBreakerEnabled                  bool   `env:"OPENCSG_AIGATEWAY_CIRCUIT_BREAKER_ENABLED" default:"true"`
		CircuitBreakerFailureThreshold         int    `env:"OPENCSG_AIGATEWAY_CIRCUIT_BREAKER_FAILURE_THRESHOLD" default:"3"`
		CircuitBreakerOpenDuration             int    `env:"OPENCSG_AIGATEWAY_CIRCUIT_BREAKER_OPEN_DURATION" default:"30"`
		CircuitBreakerHalfOpenMax              int    `env:"OPENCSG_AIGATEWAY_CIRCUIT_BREAKER_HALF_OPEN_MAX" default:"1"`
		AsyncGenerationStatusRefreshInterval   int    `env:"OPENCSG_AIGATEWAY_ASYNC_GENERATION_STATUS_REFRESH_INTERVAL" default:"60"`
		AsyncGenerationMeteringBatchSize       int    `env:"OPENCSG_AIGATEWAY_ASYNC_GENERATION_METERING_BATCH_SIZE" default:"100"`
		AsyncGenerationMaxAge                  int    `env:"OPENCSG_AIGATEWAY_ASYNC_GENERATION_MAX_AGE_SECONDS" default:"86400"`
		// CapacityPolicyDefaults supplies fallback limits for per-upstream
		// CapacityPolicy. When an admin enables CapacityPolicy on an upstream
		// but leaves every limit unset (all zeros), the gateway applies these
		// defaults wholesale on every read of upstream configs (not once at
		// load time; the defaults themselves are read from the config when
		// the gateway component is constructed). A default of <= 0 keeps
		// "no limit" for that dimension.
		CapacityPolicyDefaults struct {
			MaxConcurrency   int   `env:"OPENCSG_AIGATEWAY_CAPACITY_POLICY_DEFAULT_MAX_CONCURRENCY" default:"32"`
			MaxQueueDepth    int   `env:"OPENCSG_AIGATEWAY_CAPACITY_POLICY_DEFAULT_MAX_QUEUE_DEPTH" default:"16"`
			MaxTPM           int64 `env:"OPENCSG_AIGATEWAY_CAPACITY_POLICY_DEFAULT_MAX_TPM" default:"10000000"`
			MaxRPM           int   `env:"OPENCSG_AIGATEWAY_CAPACITY_POLICY_DEFAULT_MAX_RPM" default:"100"`
			QueueWaitSeconds int   `env:"OPENCSG_AIGATEWAY_CAPACITY_POLICY_DEFAULT_QUEUE_WAIT_SECONDS" default:"60"`
		}
		// CapacityAdmission configures the request admission control that
		// enforces per-upstream CapacityPolicy limits (concurrency, RPM and
		// TPM with pre-reservation) in Redis before requests are forwarded
		// to the runtime. All state is shared across gateway replicas.
		CapacityAdmission struct {
			// LeaseTTLSeconds is the lease expiry used for the distributed
			// concurrency/TPM reservation. Active requests renew the lease
			// every TTL/3; leases left by crashed replicas are reclaimed
			// (together with their TPM reservation) within TTL.
			LeaseTTLSeconds int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_LEASE_TTL" default:"60"`
			// FailOpen admits requests without a lease when Redis is
			// unavailable (protection degrades instead of blocking traffic).
			FailOpen bool `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_FAIL_OPEN" default:"true"`
			// EstimatePromptCharsPerToken converts prompt text length into
			// an estimated token count for the TPM pre-reservation (soft
			// heuristic; corrected with real usage after the response).
			EstimatePromptCharsPerToken int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_ESTIMATE_CHARS_PER_TOKEN" default:"4"`
			// EstimateCompletionTokens is the per-request completion token
			// reserve added on top of the prompt estimate.
			EstimateCompletionTokens int64 `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_ESTIMATE_COMPLETION_TOKENS" default:"1000"`
			// EstimateMaxTokens caps the per-request TPM reservation so an
			// oversized prompt cannot exhaust the whole TPM window.
			EstimateMaxTokens int64 `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_ESTIMATE_MAX_TOKENS" default:"32768"`
			// RetryAfterHintSeconds is the Retry-After hint returned when a
			// request is rejected because concurrency is exhausted. It is a
			// hint, not a capacity guarantee.
			RetryAfterHintSeconds int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_RETRY_AFTER_HINT" default:"5"`
			// QueueFallbackPollMs is the per-waiter fallback polling interval
			// for the admission reservation queue. Redis Pub/Sub wake-ups are
			// only an acceleration; this poll guarantees eventual progress
			// when a wake-up message is lost. Each tick also drives the
			// scheduler (promote-ready) so slots freed by expired-lease
			// cleanup (crashed replicas) make progress without a release
			// event.
			QueueFallbackPollMs int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_QUEUE_FALLBACK_POLL_MS" default:"1000"`
			// QueueClaimDeadlineSeconds bounds the window between the
			// scheduler granting a queued ticket its admission lease and the
			// waiting request goroutine taking ownership of it (claim). A
			// grant that is never claimed (crashed replica, lost wake-up
			// beyond the poll interval) expires within this window and the
			// slot is reclaimed by the expired-lease cleanup.
			QueueClaimDeadlineSeconds int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_QUEUE_CLAIM_DEADLINE" default:"5"`
			// QueueMaxRePlans bounds how many times the planner re-runs
			// Router→Admission when a queued ticket is cancelled because its
			// upstream turned unavailable while waiting. The queue never
			// picks another upstream itself; the full plan restart does. The
			// final failure renders as model_unavailable (503).
			QueueMaxRePlans int `env:"OPENCSG_AIGATEWAY_CAPACITY_ADMISSION_QUEUE_MAX_REPLANS" default:"1"`
		}
		// PIICheckEnabled turns on sensitive-content detection (PII, credentials)
		// on request prompts. Detection may rewrite what reaches the upstream
		// (mask/skip today, block later), so it only ever runs when the
		// deployment enables it AND the client opts in per request with the
		// X-PII-Check header — the gateway never rewrites payloads silently.
		// Rules live in the aigateway_pii_rules table and merge over the
		// compiled-in defaults by name.
		PIICheckEnabled bool `env:"OPENCSG_AIGATEWAY_PII_CHECK_ENABLED" default:"false"`
		// PIICheckRefreshIntervalSeconds is how often the gateway reloads
		// the rule table in the background (lazy, triggered by requests; the
		// serving request is never blocked). 0 disables refresh: rules load
		// once at startup.
		PIICheckRefreshIntervalSeconds  int `env:"OPENCSG_AIGATEWAY_PII_CHECK_REFRESH_INTERVAL_SECONDS" default:"300"`
		MetricsCollectorLookbackMinutes int `env:"OPENCSG_AIGATEWAY_METRICS_COLLECTOR_LOOKBACK_MINUTES" default:"60"`

		// SemanticRouter configures EE/SaaS automatic model selection. CE
		// ignores this configuration. When
		// ServerURL is set and the service answers its health check, the
		// gateway publishes an extra virtual model (ModelID) that clients
		// can request instead of naming a concrete model; the service then
		// ranks the real candidates for each turn and the highest ranked
		// available one serves the request.  Leaving ServerURL empty
		// disables the feature entirely and the virtual model is not
		// published.
		SemanticRouter struct {
			ExplorationEnabled       bool   `env:"OPENCSG_AIGATEWAY_ROUTER_EXPLORATION_ENABLED" default:"false"`
			FeedbackEnabled          bool   `env:"OPENCSG_AIGATEWAY_ROUTER_FEEDBACK_ENABLED" default:"false"`
			FeedbackShardingEnabled  bool   `env:"OPENCSG_AIGATEWAY_ROUTER_FEEDBACK_SHARDING_ENABLED" default:"false"`
			GenerationEnabled        bool   `env:"OPENCSG_AIGATEWAY_ROUTER_GENERATION_ENABLED" default:"true"`
			CacheEnabled             bool   `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_ENABLED" default:"false"`
			CacheEnvironment         string `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_ENVIRONMENT" default:"default"`
			CacheNamespace           string `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_NAMESPACE" default:"csghub"`
			APITokenFile             string `env:"OPENCSG_AIGATEWAY_ROUTER_API_TOKEN_FILE" default:""`
			CacheEntries             int    `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_ENTRIES" default:"4096"`
			CacheBytes               int    `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_BYTES" default:"16777216"`
			CacheFallbackConcurrency int    `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_FALLBACK_CONCURRENCY" default:"16"`
			CacheRenewConcurrency    int    `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_RENEW_CONCURRENCY" default:"4"`
			CacheRedisTimeoutMS      int    `env:"OPENCSG_AIGATEWAY_ROUTER_CACHE_REDIS_TIMEOUT_MS" default:"100"`

			ServerURL      string `env:"OPENCSG_AIGATEWAY_SEMANTIC_ROUTER_SERVER" default:""`
			ModelID        string `env:"OPENCSG_AIGATEWAY_SEMANTIC_ROUTER_MODEL_ID" default:"auto"`
			TimeoutSeconds int    `env:"OPENCSG_AIGATEWAY_SEMANTIC_ROUTER_TIMEOUT_SECONDS" default:"5"`
			// HealthTTLSeconds is how long a readiness probe result is
			// reused before the next model listing re-probes the service.
			HealthTTLSeconds int `env:"OPENCSG_AIGATEWAY_SEMANTIC_ROUTER_HEALTH_TTL_SECONDS" default:"300"`
		}

		Metrics struct {
			// Enabled controls whether the AIGateway metrics collection
			// system (DBSink and MetricCollector) is active.  When set to
			// false, per-request metrics events are discarded and the
			// per-minute collector becomes a no-op.  Disable this on
			// environments without TimescaleDB (e.g. AWS RDS) to avoid
			// write errors against hypertables converted to regular tables.
			Enabled bool `env:"OPENCSG_AIGATEWAY_METRICS_ENABLED" default:"true"`
			// DBSinkBufferSize is the buffered channel capacity used by the
			// AIGateway metrics DBSink (EE/SaaS).  When the channel is
			// full, new events are dropped with a WARN log to avoid
			// blocking request goroutines under load.
			DBSinkBufferSize int `env:"OPENCSG_AIGATEWAY_METRICS_DBSINK_BUFFER_SIZE" default:"4096"`
			// DBSinkFlushBatchSize is the maximum number of metric events
			// batched together before being flushed to TimescaleDB by the
			// DBSink background goroutine.  Also acts as upper bound on
			// slice pre-allocation in the flush loop.
			DBSinkFlushBatchSize int `env:"OPENCSG_AIGATEWAY_METRICS_DBSINK_FLUSH_BATCH_SIZE" default:"100"`
			// DBSinkFlushIntervalSeconds is the longest time (wall-clock
			// seconds) the DBSink background goroutine waits between
			// flushes, even when the current batch has fewer events than
			// DBSinkFlushBatchSize.
			DBSinkFlushIntervalSeconds int `env:"OPENCSG_AIGATEWAY_METRICS_DBSINK_FLUSH_INTERVAL_SECONDS" default:"5"`
		}
	}

	LLMLog struct {
		Bucket               string `env:"OPENCSG_LLMLOG_BUCKET" default:"opencsg-inference-logs"`
		Prefix               string `env:"OPENCSG_LLMLOG_PREFIX" default:"llmlog"`
		WorkerNum            int    `env:"OPENCSG_LLMLOG_WORKER_NUM" default:"10"`
		TaskQueueSize        int    `env:"OPENCSG_LLMLOG_TASK_QUEUE_SIZE" default:"1000"`
		UploadQueueSize      int    `env:"OPENCSG_LLMLOG_UPLOAD_QUEUE_SIZE" default:"20"`
		BatchSize            int    `env:"OPENCSG_LLMLOG_BATCH_SIZE" default:"50"`
		FlushIntervalSeconds int    `env:"OPENCSG_LLMLOG_FLUSH_INTERVAL_SECONDS" default:"60"`
		UploadTimeoutSeconds int    `env:"OPENCSG_LLMLOG_UPLOAD_TIMEOUT_SECONDS" default:"30"`
		AckWaitSeconds       int    `env:"OPENCSG_LLMLOG_ACK_WAIT_SECONDS" default:"180"`
		MaxDeliver           int    `env:"OPENCSG_LLMLOG_MAX_DELIVER" default:"6"`
		StreamMaxBytes       int64  `env:"OPENCSG_LLMLOG_STREAM_MAX_BYTES" default:"2147483648"`   // 2 GiB
		StreamMaxAgeSeconds  int    `env:"OPENCSG_LLMLOG_STREAM_MAX_AGE_SECONDS" default:"604800"` // 7 days
		Sync                 struct {
			Enable             bool     `env:"STARHUB_SERVER_LLMLOG_SYNC_ENABLE" default:"false"`
			Username           string   `env:"STARHUB_SERVER_LLMLOG_SYNC_USERNAME" default:""`
			DatasetNamespace   string   `env:"STARHUB_SERVER_LLMLOG_SYNC_DATASET_NAMESPACE" default:""`
			DatasetName        string   `env:"STARHUB_SERVER_LLMLOG_SYNC_DATASET_NAME" default:""`
			MaxShardSizeBytes  int64    `env:"STARHUB_SERVER_LLMLOG_SYNC_MAX_SHARD_SIZE_BYTES" default:"67108864"` // 64 MiB
			PreuploadBatchSize int      `env:"STARHUB_SERVER_LLMLOG_SYNC_PREUPLOAD_BATCH_SIZE" default:"10"`
			NotifyEmails       []string `env:"OPENCSG_LLMLOG_SYNC_NOTIFY_EMAILS" default:""`
			Dataflow           struct {
				Enable           bool   `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_ENABLE" default:"false"`
				ResourceScenario string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_RESOURCE_SCENARIO" default:"wf_dataflow_llmlog"`
				DatasetNamespace string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_DATASET_NAMESPACE" default:""`
				DatasetName      string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_DATASET_NAME" default:"Runtime_data_curated"`
				StorageSize      string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_STORAGE_SIZE" default:"1Gi"`
				Image            string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_IMAGE" default:"opencsghq/runtime-data-curated:latest"`
				OpenAIAPIKey     string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_OPENAI_API_KEY" default:""`
				OpenAIBaseURL    string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_OPENAI_BASE_URL" default:""`
				OpenAIModel      string `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_OPENAI_MODEL" default:""`
				LLMConcurrency   int    `env:"STARHUB_SERVER_LLMLOG_SYNC_DATAFLOW_LLM_CONCURRENCY" default:"5"`
			}
		}
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
		Port                        int    `env:"STARHUB_SERVER_NOTIFIER_PORT" default:"8095"`
		Host                        string `env:"STARHUB_SERVER_NOTIFIER_HOST" default:"http://localhost"`
		MailerHost                  string `env:"STARHUB_SERVER_MAILER_HOST" default:"smtp.qiye.aliyun.com"`
		MailerPort                  int    `env:"STARHUB_SERVER_MAILER_PORT" default:"465"`
		MailerUsername              string `env:"STARHUB_SERVER_MAILER_USERNAME" default:""`
		MailerPassword              string `env:"STARHUB_SERVER_MAILER_PASSWORD" default:""`
		DirectMailEnabled           bool   `env:"STARHUB_SERVER_DIRECT_MAIL_ENABLED" default:"false"`
		DirectMailAccessKeyID       string `env:"STARHUB_SERVER_DIRECT_MAIL_ACCESS_KEY_ID" default:""`
		DirectMailAccessKeySecret   string `env:"STARHUB_SERVER_DIRECT_MAIL_ACCESS_KEY_SECRET" default:""`
		DirectMailEndpoint          string `env:"STARHUB_SERVER_DIRECT_MAIL_ENDPOINT" default:"dm.aliyuncs.com"`
		DirectMailRegionId          string `env:"STARHUB_SERVER_DIRECT_MAIL_REGION_ID" default:"cn-hangzhou"`
		MailerRechargeAdmin         string `env:"STARHUB_SERVER_MAILER_RECHARGE_ADMIN" default:"contact@opencsg.com"`
		MailerWeeklyRechargesMail   string `env:"STARHUB_SERVER_MAILER_WEEKLY_RECHARGES_MAIL" default:"reconcile@opencsg.com"`
		EmailInvoiceCreatedReceiver string `env:"STARHUB_SERVER_EMAIL_INVOICE_CREATED_RECEIVER" default:"contact@opencsg.com"`
		RepoSyncTimezone            string `env:"STARHUB_SERVER_REPO_SYNC_TIMEZONE" default:"Asia/Shanghai"`
		RepoSyncChatID              string `env:"STARHUB_SERVER_REPO_SYNC_CHAT_ID" default:""`
		NotificationRetryCount      int    `env:"STARHUB_SERVER_NOTIFIER_NOTIFICATION_RETRY_COUNT" default:"3"`
		BroadcastUserPageSize       int    `env:"STARHUB_SERVER_NOTIFIER_BROADCAST_USER_PAGE_SIZE" default:"100"`
		BroadcastEmailPageSize      int    `env:"STARHUB_SERVER_NOTIFIER_BROADCAST_EMAIL_PAGE_SIZE" default:"100"`
		DirectEmailMaxRecipients    int    `env:"STARHUB_SERVER_NOTIFIER_DIRECT_EMAIL_MAX_RECIPIENTS" default:"5"`
		MsgDispatcherCount          int    `env:"STARHUB_SERVER_NOTIFIER_MSG_DISPATCHER_COUNT" default:"20"`
		HighPriorityMsgBufferSize   int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_BUFFER_SIZE" default:"100"`
		NormalPriorityMsgBufferSize int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_BUFFER_SIZE" default:"50"`
		HighPriorityMsgAckWait      int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_ACK_WAIT" default:"60"`
		NormalPriorityMsgAckWait    int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_ACK_WAIT" default:"60"`
		HighPriorityMsgMaxDeliver   int    `env:"STARHUB_SERVER_NOTIFIER_HIGH_PRIORITY_MSG_MAX_DELIVER" default:"6"`
		NormalPriorityMsgMaxDeliver int    `env:"STARHUB_SERVER_NOTIFIER_NORMAL_PRIORITY_MSG_MAX_DELIVER" default:"6"`
		DeduplicateWindow           int    `env:"STARHUB_SERVER_NOTIFIER_DEDUPLICATE_WINDOW" default:"5"` // 5 seconds

		// SMS Provider Configuration
		SMSProvider string `env:"STARHUB_SERVER_NOTIFIER_SMS_PROVIDER" default:"aliyun"` // aliyun, tencent, huawei

		SMSSign                             string `env:"STARHUB_SERVER_NOTIFIER_SMS_SIGN" default:""`
		SMSTemplateCodeForVerifyCodeCN      string `env:"STARHUB_SERVER_NOTIFIER_SMS_TEMPLATE_CODE_FOR_VERIFY_CODE_CN" default:""`
		SMSTemplateCodeForVerifyCodeOversea string `env:"STARHUB_SERVER_NOTIFIER_SMS_TEMPLATE_CODE_FOR_VERIFY_CODE_OVERSEA" default:""`

		// Alibaba Cloud SMS Configuration (Backward Compatibility)
		SMSAccessKeyID     string `env:"STARHUB_SERVER_NOTIFIER_SMS_ACCESS_KEY_ID"`
		SMSAccessKeySecret string `env:"STARHUB_SERVER_NOTIFIER_SMS_ACCESS_KEY_SECRET"`
		SMSRegion          string `env:"STARHUB_SERVER_NOTIFIER_SMS_REGION" default:"ap-guangzhou"`
		SMSEndpoint        string `env:"STARHUB_SERVER_NOTIFIER_SMS_ENDPOINT"`
		SMSAppID           string `env:"STARHUB_SERVER_NOTIFIER_SMS_APP_ID"` // Tencent SdkAppId / Huawei ProjectId
	}

	Prometheus struct {
		ApiAddress           string   `env:"STARHUB_SERVER_PROMETHEUS_API_ADDRESS" default:""`
		BasicAuth            string   `env:"STARHUB_SERVER_PROMETHEUS_BASIC_AUTH" default:""`
		InsecureSkipVerify   bool     `env:"STARHUB_SERVER_PROMETHEUS_INSECURE_SKIP_VERIFY" default:"false"`
		CPUUsageMetric       string   `env:"STARHUB_SERVER_PROMETHEUS_CPU_USAGE_METRIC" default:"container_cpu_usage_seconds_total"`
		CPULimitMetric       string   `env:"STARHUB_SERVER_PROMETHEUS_CPU_LIMIT_METRIC" default:"kube_pod_container_resource_limits"`
		MemoryUsageMetric    string   `env:"STARHUB_SERVER_PROMETHEUS_MEMORY_USAGE_METRIC" default:"container_memory_usage_bytes"`
		RequestCountMetric   string   `env:"STARHUB_SERVER_PROMETHEUS_REQUEST_COUNT_METRIC" default:"revision_request_count"`
		RequestLatencyMetric string   `env:"STARHUB_SERVER_PROMETHEUS_REQUEST_LATENCY_METRIC" default:"revision_app_request_latencies_bucket"`
		MetricKeys           []string `env:"STARHUB_SERVER_PROMETHEUS_METRIC_KEYS" default:"[pod,service_name,namespace,http_response_status_code,response_code_class,le]"`
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
		ResourceQuotaNamespace       string `env:"STARHUB_SERVER_CLUSTER_RESOURCE_QUOTA_NAMESPACE" default:"spaces"`
		QuotaName                    string `env:"STARHUB_SERVER_CLUSTER_QUOTA_NAME" default:""`
		AllowCPUResScheduleToGPUNode bool   `env:"STARHUB_SERVER_CLUSTER_ALLOW_CPU_RESOURCE_SCHEDULE_TO_GPU_NODE" default:"false"`
	}

	Runner struct {
		ImageBuilderGitImage    string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_GIT_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/alpine/git:2.36.2"`
		ImageBuilderKanikoImage string   `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/public/kaniko-project-executor:v1.23.2"`
		ImageBuilderJobTTL      int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_JOB_TTL" default:"120"`
		ImageBuilderStatusTTL   int      `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_STATUS_TTL" default:"300"`
		ImageBuilderKanikoArgs  []string `env:"STARHUB_SERVER_RUNNER_IMAGE_BUILDER_KANIKO_ARGS"`
		SystemCUDAVersion       string   `env:"STARHUB_SERVER_RUNNER_SYSTEM_CUDA_VERSION" default:""`
		// ReclaimImage is the container image used by the one-shot reclaim Job that
		// deletes a sandbox's PVC subPath directory when the sandbox is deleted.
		// Only requirement: a POSIX shell + rm. Default points to the internal
		// registry so offline clusters don't depend on Docker Hub.
		ReclaimImage string `env:"STARHUB_SERVER_RUNNER_RECLAIM_IMAGE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/busybox:1.36"`
		// csghub server webhook endpoint
		WebHookEndpoint    string `env:"STARHUB_SERVER_RUNNER_WEBHOOK_ENDPOINT" default:"http://localhost:8080"`
		WatchConfigmapName string `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_NAME" default:"spaces-runner-config"`
		// WatchConfigmapKey           string `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_KEY" default:""`
		WatchConfigmapIntervalInSec int    `env:"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_INTERVAL_IN_SEC" default:"60"`
		HearBeatIntervalInSec       int    `env:"STARHUB_SERVER_RUNNER_HEARTBEAT_INTERVAL_IN_SEC" default:"120"`
		RunnerNamespace             string `env:"STARHUB_SERVER_CLUSTER_RUNNER_NAMESPACE" default:"csghub"`
		KubeScheduler               string `env:"STARHUB_SERVER_RUNNER_KUBE_SCHEDULER" default:""`
		Volcano                     struct {
			Queue string `env:"STARHUB_SERVER_RUNNER_VOLCANO_QUEUE" default:"default"`
		}
		PublicDockerRegBase     string `env:"STARHUB_SERVER_RUNNER_PUBLIC_DOCKER_REG_BASE" default:"opencsg-registry.cn-beijing.cr.aliyuncs.com"`
		VGPUNodeResourceName    string `env:"STARHUB_SERVER_RUNNER_VGPU_NODE_RESOURCE_NAME" default:"hami.io/node-nvidia-register"`
		VGPUPodResourceName     string `env:"STARHUB_SERVER_RUNNER_VGPU_POD_RESOURCE_NAME" default:"hami.io/vgpu-devices-allocated"`
		VGPUResourceReqKey      string `env:"STARHUB_SERVER_RUNNER_VGPU_RESOURCE_REQ_KEY" default:"nvidia.com/vgpu"`
		VGPUMemoryReqKey        string `env:"STARHUB_SERVER_RUNNER_VGPU_MEMORY_REQ_KEY" default:"nvidia.com/vgpumem"`
		VGPUMemoryScalingFactor int64  `env:"STARHUB_SERVER_RUNNER_VGPU_MEMORY_SCING_FACTOR" default:"10"`
		GPUModelLabel           string `env:"STARHUB_SERVER_RUNNER_GPU_MODEL_LABEL" default:""`
		PD                      struct {
			// EPPImage is the image ID for the Endpoint Picker deployment. Only the short image ID is configured;
			// it will be resolved to a full image path via resolveContainerImage at runtime.
			EPPImage string `env:"STARHUB_SERVER_RUNNER_PD_EPP_IMAGE" default:"opencsghq/llm-d/llm-d-router-endpoint-picker-dev:main"`
			// RoutingProxyImage is the image ID for the routing-proxy sidecar. Only the short image ID is configured;
			// it will be resolved to a full image path via resolveContainerImage at runtime.
			RoutingProxyImage string `env:"STARHUB_SERVER_RUNNER_PD_ROUTING_PROXY_IMAGE" default:"opencsghq/llm-d/llm-d-routing-sidecar:v0.8.0"`
			// EPPServiceAccountName is the namespace-level SA that EPP pods should use.
			EPPServiceAccountName string `env:"STARHUB_SERVER_RUNNER_PD_EPP_SERVICE_ACCOUNT_NAME" default:"llm-d-epp"`
			// EnvoyImage is the Envoy sidecar image for EPP HTTP proxy (port 8081).
			EnvoyImage string `env:"STARHUB_SERVER_RUNNER_PD_ENVOY_IMAGE" default:"opencsghq/envoyproxy/envoy:distroless-v1.37.2"`
			// EnvoyConfigMapName is the global ConfigMap name containing envoy.yaml for all PD EPP deployments.
			EnvoyConfigMapName string `env:"STARHUB_SERVER_RUNNER_PD_ENVOY_CONFIGMAP_NAME" default:"pd-epp-envoy"`
			// EPPConfigMapName is the global ConfigMap name containing pd-config.yaml for all PD EPP deployments.
			EPPConfigMapName string `env:"STARHUB_SERVER_RUNNER_PD_EPP_CONFIGMAP_NAME" default:"pd-epp-config"`
		}
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
		QueryLastReportTimeout int    `env:"STARHUB_SERVER_LOGCOLLECTOR_QUERY_LAST_REPORT_TIMEOUT" default:"300"`
	}

	FederationAdapter struct {
		Host string `env:"STARHUB_SERVER_FEDERATION_ADAPTER_ENDPOINT" default:"http://localhost"`
		Port int    `env:"STARHUB_SERVER_FEDERATION_ADAPTER_PORT" default:"8099"`
	}

	TrustRegistry struct {
		Host string `env:"STARHUB_SERVER_FEDERATION_REGISTRY_HOST" default:"http://localhost"`
		Port int    `env:"STARHUB_SERVER_FEDERATION_REGISTRY_PORT" default:"8098"`
	}

	Temporal struct {
		MaxConcurrentActivityExecutionSize      int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_ACTIVITY_EXECUTION_SIZE" default:"5"`
		MaxConcurrentLocalActivityExecutionSize int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_LOCAL_ACTIVITY_EXECUTION_SIZE" default:"10"`
		MaxConcurrentWorkflowTaskExecutionSize  int `env:"OPENCSG_TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASK_EXECUTION_SIZE" default:"50"`
		GetSystemInfoTimeout                    int `env:"OPENCSG_TEMPORAL_GET_SYSTEM_INFO_TIMEOUT" default:"5"`
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
		Endpoint                string `env:"STARHUB_SERVER_XNET_ENDPOINT" default:"http://localhost:8097"`
		ApiKey                  string `env:"STARHUB_SERVER_XNET_API_KEY" default:"f3a7b9c1d6e5f8e2a1b5d4f9e6a2b8d7c3a4e2b1d9f6e7a8d2c5a7b4c1e3f5b8a1d4f9b7d6e2f8a5d3b1e7f9c6a8b2d1e4f7d5b6e9f2a4b3c8e1d7f995hd82hf"`
		MigrationBatchSize      int    `env:"STARHUB_SERVER_XNET_MIGRATION_BATCH_SIZE" default:"100"`
		MigrationMaxInFlight    int    `env:"STARHUB_SERVER_XNET_MIGRATION_MAX_IN_FLIGHT" default:"1000"`
		MigrationPublishRate    int    `env:"STARHUB_SERVER_XNET_MIGRATION_PUBLISH_RATE" default:"50"`
		MigrationLeaseSeconds   int    `env:"STARHUB_SERVER_XNET_MIGRATION_LEASE_SECONDS" default:"300"`
		MigrationPollSeconds    int    `env:"STARHUB_SERVER_XNET_MIGRATION_POLL_SECONDS" default:"5"`
		MigrationMaxAttempts    int    `env:"STARHUB_SERVER_XNET_MIGRATION_MAX_ATTEMPTS" default:"5"`
		MigrationStreamMaxMsgs  int64  `env:"STARHUB_SERVER_XNET_MIGRATION_STREAM_MAX_MSGS" default:"1000"`
		MigrationStreamMaxBytes int64  `env:"STARHUB_SERVER_XNET_MIGRATION_STREAM_MAX_BYTES" default:"1073741824"`
	}

	StorageGateway struct {
		PartSize                int64 `env:"STARHUB_SERVER_STORAGE_GATEWAY_PART_SIZE" default:"67108864"`              // 64MB
		EnablePresignedURLProxy bool  `env:"STARHUB_SERVER_STORAGE_GATEWAY_ENABLE_PRESIGNED_URL_PROXY" default:"true"` // Enable presigned URL proxy through gateway
		EnableUploadViaGateway  bool  `env:"STARHUB_SERVER_STORAGE_GATEWAY_ENABLE_UPLOAD_VIA_GATEWAY" default:"false"` // Enable file uploads through storage gateway proxy in evaluation containers
	}

	AuditLog struct {
		Enabled          bool     `env:"STARHUB_SERVER_AUDIT_LOG_ENABLE" default:"false"`
		SkipRecordTables []string `env:"STARHUB_SERVER_AUDIT_LOG_SKIP_RECORD_TABLES" default:"[]"`
	}

	Skill struct {
		MaxPublishFileCount int   `env:"STARHUB_SERVER_SKILL_MAX_PUBLISH_FILE_COUNT" default:"100"`
		MaxPublishFileSize  int64 `env:"STARHUB_SERVER_SKILL_MAX_PUBLISH_FILE_SIZE" default:"10485760"` // 10MB
	}

	ActivityLog struct {
		Enabled bool `env:"STARHUB_SERVER_ACTIVITY_LOG_ENABLE" default:"true"`
	}

	MaxRepoBatchNum int `env:"STARHUB_SERVER_MAX_REPO_SIZE_BATCH_NUM" default:"500"`
}

type MemoryConfig struct {
	Enable           bool   `env:"OPENCSG_MEMORY_SERVER_ENABLED" default:"true"`
	Backend          string `env:"OPENCSG_MEMORY_BACKEND" default:"memmachine"`
	Host             string `env:"OPENCSG_MEMORY_SERVER_HOST" default:"http://memmachine"`
	Port             int    `env:"OPENCSG_MEMORY_SERVER_PORT" default:"8080"`
	BasePath         string `env:"OPENCSG_MEMORY_SERVER_BASE_PATH" default:"/api/v2"`
	ApiKey           string `env:"OPENCSG_MEMORY_SERVER_API_KEY" default:""`
	TimeoutSeconds   int    `env:"OPENCSG_MEMORY_SERVER_TIMEOUT_SECONDS" default:"10"`
	RetryCount       int    `env:"OPENCSG_MEMORY_SERVER_RETRY_COUNT" default:"1"`
	RetryDelayMillis int    `env:"OPENCSG_MEMORY_SERVER_RETRY_DELAY_MILLIS" default:"100"`
}

func SetConfigFile(file string) {
	configFile = file
}

var globalConfig *Config
var globalConfigError error
var _globalLocation *time.Location // private in package
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
	if err != nil {
		return nil, err
	}

	if len(cfg.UniqueServiceName) < 1 {
		cfg.UniqueServiceName = genServiceName()
	}

	cfg.S3.Endpoint = NormalizeS3Endpoint(cfg.S3.Endpoint, cfg.S3.EnableDualstack, cfg.S3.Region)
	cfg.S3.InternalEndpoint = NormalizeS3Endpoint(cfg.S3.InternalEndpoint, cfg.S3.EnableDualstack, cfg.S3.Region)

	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", cfg.TimeZone, err)
	}
	SetGlobalTimeZone(loc)
	slog.Info("Init server timezone", slog.Any("config", cfg.TimeZone), slog.Any("using", _globalLocation.String()))

	return cfg, nil
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

func GetGlobalTimeZone() *time.Location {
	if _globalLocation == nil {
		return time.Local
	}
	return _globalLocation
}

// SetGlobalTimeZone sets the global timezone location used by GetGlobalTimeZone.
// This is primarily useful for tests that need to verify timezone-aware behavior
// without calling LoadConfig.
func SetGlobalTimeZone(loc *time.Location) {
	_globalLocation = loc
}
