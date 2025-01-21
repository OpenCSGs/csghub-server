# Instrumenting CSGHub Server

This guide provides conventions and best practices for instrumenting the CSGHub server using logs, metrics, and traces.

## Enable Instrumentation Collection

CSGHub Server uses OpenTelemetry to collect instrumentation data. To enable this, set the `instrumentation.otlp_endpoint` in the config file to your OpenTelemetry collector's address. For logging, we use the `slogs` package. If you enable `instrumentation.otlp_logging`, an extra `otelslog` handler will be attached to the logger and logs will be collected by the OpenTelemetry collector. The code which setup OpenTelemetry SDK can be found here: builder/instrumentation/otel.go.

For a quick setup to test and visualize logs, traces, and metrics locally, see the [Start Grafana LGTM and Visualize Logs/Traces/Metrics Locally](#start-grafana-lgtm-and-visualize-logstracesmetrics-locally) section.

## Logs

Logs provide important information about the application's behavior and any potential issues. They are useful for debugging and monitoring the system.

### Usage

You can log messages directly using the `slog` package:

```go
import (
  "fmt"
  "log/slog"
)

slog.Info("Info message")
slog.Warning("Warning message")
slog.Error("Error message", "error", fmt.Errorf("BOOM"))
```

You can also log with context, which will include the current span identifier when tracing is enabled:

```go
import (
  "fmt"
  "log/slog"
)

slog.InfoContext(ctx, "Info message with context")
```

## Metrics

Metrics provide quantitative data about the application's performance and resource usage. They are essential for monitoring the health and performance of the server.

### Usage

CSGHub Server automatically reports Go runtime metrics to the OpenTelemetry endpoint when enabled. You can also report custom metrics manually.

## Traces

Traces are used for distributed tracing to help understand the flow of requests and operations across various services.

### Usage

The `otelgin` middleware is used to automatically collect traces for all API requests. Additionally, database tracing and git gRPC tracing are enabled automatically.

You can also create custom spans manually. For example:

```go
import (
   "fmt"

   "go.opentelemetry.io/otel/attribute"
   "go.opentelemetry.io/otel/trace"
   "go.opentelemetry.io/otel"
   "go.opentelemetry.io/otel/codes"
)

var (
	tracer = otel.Tracer("test")
)

type MyService struct {}

func ProvideService() *MyService {
   return &MyService{}
}

func (s *MyService) Hello(ctx context.Context, name string) (string, error) {
   ctx, span := tracer.Start(ctx, "MyService.Hello", trace.WithAttributes(
      attribute.String("my_attribute", "val"),
   ))
   // make sure the span is marked as finished when this
   // method ends to allow the span to be flushed and sent to
   // storage backend.
   defer span.End()

   // Add some event to show Events usage
   span.AddEvent("checking name...")

   if name == "" {
      err := fmt.Errorf("name cannot be empty")
	  span.SetStatus(codes.Error, "failed")
	  span.RecordError(err)

      return "", err
   }

   // Add some other event to show Events usage
   span.AddEvent("name checked")

   // Add attribute to show Attributes usage
   span.SetAttributes(
      attribute.String("my_service.name", name),
      attribute.Int64("my_service.some_other", int64(1337)),
   )

   return fmt.Sprintf("Hello %s", name), nil
}
```

## Start Grafana LGTM and Visualize Logs/Traces/Metrics Locally

1. **Start Grafana LGTM using Docker or Docker Compose**
   You can start Grafana LGTM by following the Docker or Docker Compose instructions.
   - Docker: https://github.com/grafana/docker-otel-lgtm
   - Docker Compose: builder/instrumentation/compose-example/docker-compose.yaml
   
  This setup includes the OpenTelemetry Collector, Grafana, Prometheus, Loki, and Tempo. For more detailed instructions, please refer to the docker-otel-lgtm README.

2. **Set the OTLP Endpoint in the Config**
   To enable reporting data to OpenTelemetry Collector, set the following in your config TOML file:

   ```toml
   [instrumentation]
   otlp_endpoint = "http://localhost:4317"
   otlp_logging = true
   ```

   Then, restart the CSGHub server.

3. **Browse Logs/Traces/Metrics in Grafana**
   Open [http://localhost:3000](http://localhost:3000) to view the collected logs, traces, and metrics. You can also import example dashboards here: builder/instrumentation/grafana-dashboards-example.
