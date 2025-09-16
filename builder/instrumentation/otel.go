package instrumentation

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"log/slog"
	"net/url"
	"time"

	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	olog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"opencsg.com/csghub-server/common/config"
)

func convEndpoint(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func convInsecure(s string) (bool, error) {
	u, err := url.Parse(s)
	if err != nil {
		return true, err
	}

	if u.Scheme == "https" {
		return false, nil
	}
	return true, nil
}

func SetupOTelSDK(ctx context.Context, config *config.Config, serviceName string) (func(context.Context) error, error) {
	if config.Instrumentation.OTLPEndpoint == "" {
		return func(ctx context.Context) error {
			return nil
		}, nil
	}
	endpoint, err := convEndpoint(config.Instrumentation.OTLPEndpoint)
	if err != nil {
		return nil, err
	}
	insecure, err := convInsecure(config.Instrumentation.OTLPEndpoint)
	if err != nil {
		return nil, err
	}

	var shutdownFuncs []func(context.Context) error

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	res, err := resource.New(context.Background(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithHost(),
		resource.WithContainer(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptrace.New(
		ctx, otlptracegrpc.NewClient(options...),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	if err != nil {
		handleErr(err)
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	optionsMetric := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
	}
	if insecure {
		optionsMetric = append(optionsMetric, otlpmetricgrpc.WithInsecure())
	}
	metricExporter, err := otlpmetricgrpc.New(
		ctx, optionsMetric...,
	)
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)
	if err != nil {
		handleErr(err)
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	if config.Instrumentation.OTLPLogging {
		optionsLog := []otlploggrpc.Option{
			otlploggrpc.WithEndpoint(endpoint),
		}
		if insecure {
			optionsLog = append(optionsLog, otlploggrpc.WithInsecure())
		}
		logExporter, err := otlploggrpc.New(
			ctx, optionsLog...,
		)
		if err != nil {
			return nil, err
		}

		loggerProvider := olog.NewLoggerProvider(
			olog.WithProcessor(olog.NewBatchProcessor(logExporter)),
			olog.WithResource(res),
		)
		handlers := []slog.Handler{
			slog.Default().Handler(),
			otelslog.NewHandler("csghub-server"),
		}
		slog.SetDefault(slog.New(slogmulti.Fanout(handlers...)))

		shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
		global.SetLoggerProvider(loggerProvider)
	}

	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(10 * time.Second))
	if err != nil {
		handleErr(err)
		return nil, err
	}

	return shutdown, nil
}

// SetupOtelMiddleware sets up the otelgin middleware for the gin engine.
func SetupOtelMiddleware(r *gin.Engine, config *config.Config, serviceName string) {
	if config.Instrumentation.OTLPEndpoint != "" {
		r.Use(otelgin.Middleware(serviceName))
	}
}
