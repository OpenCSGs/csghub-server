module opencsg.com/csghub-server

go 1.24.6

require (
	github.com/DATA-DOG/go-txdb v0.2.0
	github.com/OpenCSGs/gitea-go-sdk/gitea v0.0.0-20240618091626-54fa52f1cec6
	github.com/alibabacloud-go/darabonba-openapi/v2 v2.1.8
	github.com/alibabacloud-go/dm-20151123/v2 v2.4.0
	github.com/alibabacloud-go/dysmsapi-20170525/v5 v5.1.2
	github.com/alibabacloud-go/green-20220302/v2 v2.20.0
	github.com/alibabacloud-go/tea v1.3.9
	github.com/aliyun/alibaba-cloud-sdk-go v1.62.648
	github.com/aliyun/aliyun-oss-go-sdk v3.0.2+incompatible
	github.com/andybalholm/brotli v1.1.0
	github.com/avast/retry-go/v4 v4.6.1
	github.com/blang/semver/v4 v4.0.0
	github.com/bmatcuk/doublestar/v4 v4.8.1
	github.com/casdoor/casdoor-go-sdk v1.22.0
	github.com/chenyahui/gin-cache v1.9.0
	github.com/d5/tengo/v2 v2.17.0
	github.com/gin-contrib/cors v1.7.0
	github.com/gin-contrib/pprof v1.5.1
	github.com/gin-contrib/sessions v0.0.5
	github.com/gin-gonic/gin v1.10.0
	github.com/go-pay/errgroup v0.0.2
	github.com/go-redsync/redsync/v4 v4.13.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/golang/mock v1.6.0
	github.com/google/wire v0.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/jarcoal/httpmock v1.3.1
	github.com/looplab/fsm v1.0.3
	github.com/marcboeker/go-duckdb v1.5.6
	github.com/matoous/go-nanoid/v2 v2.1.0
	github.com/mcuadros/go-defaults v1.2.0
	github.com/minio/minio-go/v7 v7.0.84
	github.com/minio/sha256-simd v1.0.1
	github.com/naoina/toml v0.1.1
	github.com/nicksnyder/go-i18n/v2 v2.6.0
	github.com/nyaruka/phonenumbers v1.6.5
	github.com/openai/openai-go/v3 v3.8.1
	github.com/oschwald/geoip2-golang/v2 v2.0.0
	github.com/parquet-go/parquet-go v0.24.0
	github.com/prometheus/client_golang v1.21.1
	github.com/redis/go-redis/v9 v9.5.5
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/samber/slog-multi v1.3.3
	github.com/sethvargo/go-envconfig v1.1.0
	github.com/spf13/cast v1.7.0
	github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify v1.11.1
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/swag v1.16.2
	github.com/testcontainers/testcontainers-go v0.40.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.40.0
	github.com/tidwall/sjson v1.2.5
	github.com/uptrace/bun v1.2.8
	github.com/uptrace/bun/dialect/pgdialect v1.2.8
	github.com/uptrace/bun/dialect/sqlitedialect v1.2.8
	github.com/uptrace/bun/driver/pgdriver v1.2.8
	github.com/uptrace/bun/driver/sqliteshim v1.2.8
	github.com/uptrace/bun/extra/bundebug v1.2.8
	github.com/uptrace/bun/extra/bunotel v1.2.8
	gitlab.com/gitlab-org/gitaly/v16 v16.11.5
	go.opentelemetry.io/contrib/bridges/otelslog v0.8.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.58.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0
	go.opentelemetry.io/contrib/instrumentation/runtime v0.58.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.9.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
	go.opentelemetry.io/otel/log v0.9.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/otel/sdk/log v0.9.0
	go.opentelemetry.io/otel/sdk/metric v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.temporal.io/api v1.46.0
	go.temporal.io/sdk v1.34.0
	go.temporal.io/sdk/contrib/opentelemetry v0.6.0
	go.temporal.io/server v1.27.2
	google.golang.org/grpc v1.71.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.32.3
	k8s.io/apimachinery v0.32.3
	k8s.io/client-go v0.32.3
	knative.dev/serving v0.43.0
)

require (
	cel.dev/expr v0.22.1 // indirect
	cloud.google.com/go v0.120.0 // indirect
	cloud.google.com/go/auth v0.15.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.4.2 // indirect
	cloud.google.com/go/monitoring v1.24.1 // indirect
	cloud.google.com/go/storage v1.51.0 // indirect
	cloud.google.com/go/trace v1.11.3 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.14 // indirect
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/DataDog/datadog-go v4.4.0+incompatible // indirect
	github.com/DataDog/sketches-go v1.0.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.51.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.51.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/apache/thrift v0.21.0 // indirect
	github.com/aws/aws-sdk-go v1.55.6 // indirect
	github.com/beevik/ntp v1.3.1 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic/loader v0.2.1 // indirect
	github.com/cactus/go-statsd-client/statsd v0.0.0-20200423205355-cb0885a1018c // indirect
	github.com/cactus/go-statsd-client/v5 v5.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250121191232-2f005788dc42 // indirect
	github.com/containerd/containerd v1.7.27 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.5.1+incompatible // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-sql-driver/mysql v1.9.1 // indirect
	github.com/gocql/gocql v1.7.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/yamux v0.1.2-0.20220728231024-8f49b6f63f18 // indirect
	github.com/henvic/httpretty v0.1.4 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.4 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lightstep/lightstep-tracer-common/golang/gogo v0.0.0-20210210170715-a8dfcb80d3a7 // indirect
	github.com/lightstep/lightstep-tracer-go v0.25.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.1.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/naoina/go-stringutil v0.1.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nexus-rpc/sdk-go v0.3.0 // indirect
	github.com/oklog/ulid/v2 v2.0.2 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/olivere/elastic/v7 v7.0.32 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/oschwald/maxminddb-golang/v2 v2.0.0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/prometheus/prometheus v0.50.1 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.4.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rs/dnscache v0.0.0-20230804202142-fc85eb664529 // indirect
	github.com/samber/lo v1.47.0 // indirect
	github.com/sebest/xff v0.0.0-20210106013422-671bd2870b3a // indirect
	github.com/shirou/gopsutil/v3 v3.23.12 // indirect
	github.com/shirou/gopsutil/v4 v4.25.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/smallnest/ringbuffer v0.0.0-20241116012123-461381446e3d // indirect
	github.com/sony/gobreaker v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/temporalio/ringpop-go v0.0.0-20250130211428-b97329e994f7 // indirect
	github.com/temporalio/sqlparser v0.0.0-20231115171017-f4060bcfa6cb // indirect
	github.com/temporalio/tchannel-go v1.22.1-0.20240528171429-1db37fdea938 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/uber-common/bark v1.3.0 // indirect
	github.com/uber-go/tally/v4 v4.1.17-0.20240412215630-22fe011f5ff0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/uptrace/opentelemetry-go-extra/otelsql v0.3.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	gitlab.com/gitlab-org/go/reopen v1.0.0 // indirect
	gitlab.com/gitlab-org/labkit v1.21.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.57.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.temporal.io/version v0.3.0 // indirect
	go.uber.org/dig v1.18.1 // indirect
	go.uber.org/fx v1.23.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/api v0.228.0 // indirect
	google.golang.org/genproto v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	gopkg.in/DataDog/dd-trace-go.v1 v1.32.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	gopkg.in/validator.v2 v2.0.1 // indirect
	modernc.org/gc/v3 v3.0.0 // indirect
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/XSAM/otelsql v0.36.0
	github.com/alibabacloud-go/alibabacloud-gateway-spi v0.0.5 // indirect
	github.com/alibabacloud-go/debug v1.0.1 // indirect
	github.com/alibabacloud-go/endpoint-util v1.1.1 // indirect
	github.com/alibabacloud-go/openapi-util v0.1.1 // indirect
	github.com/alibabacloud-go/tea-utils/v2 v2.0.7
	github.com/aliyun/credentials-go v1.4.5
	github.com/argoproj/argo-workflows/v3 v3.5.13
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/bwmarrin/snowflake v0.3.0
	github.com/bytedance/sonic v1.12.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/clbanning/mxj/v2 v2.7.0 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.11.0
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.7 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-fed/httpsig v1.1.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/spec v0.20.14 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.23.0
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/goccy/go-json v0.10.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.16.1 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/gorilla/sessions v1.2.1 // indirect
	github.com/gpustack/gguf-parser-go v0.17.2
	github.com/hashicorp/go-version v1.7.0
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jellydator/ttlcache/v2 v2.11.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.1-0.20220621161143-b0104c826a24 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nats.go v1.35.0
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.1-0.20220228012449-10b1cf09e00b // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stripe/stripe-go/v82 v82.3.0
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/arch v0.12.0 // indirect
	golang.org/x/crypto v0.43.0
	golang.org/x/net v0.45.0 // indirect
	golang.org/x/oauth2 v0.28.0
	golang.org/x/sync v0.17.0
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
	golang.org/x/text v0.30.0
	golang.org/x/time v0.11.0
	golang.org/x/tools v0.37.0
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/utils v0.0.0-20241210054802-24370beab758
	knative.dev/networking v0.0.0-20241022012959-60e29ff520dc // indirect
	knative.dev/pkg v0.0.0-20241021183759-9b9d535af5ad // indirect
	mellium.im/sasl v0.3.2 // indirect
	modernc.org/libc v1.61.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.9.1 // indirect
	modernc.org/sqlite v1.34.4 // indirect
	modernc.org/strutil v1.2.1 // indirect
	modernc.org/token v1.1.0 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/lws v0.6.1
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
