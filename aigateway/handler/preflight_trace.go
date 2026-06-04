package handler

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
)

const requestPreflightTracerName = "opencsg.com/csghub-server/aigateway/preflight"

type preflightTraceStart struct {
	API       string
	RequestID string
	UserID    string
}

type preflightTrace struct {
	span    oteltrace.Span
	endOnce sync.Once
}

func startPreflightTrace(ctx context.Context, input preflightTraceStart) (context.Context, *preflightTrace) {
	if ctx == nil {
		ctx = context.Background()
	}
	attrs := make([]attribute.KeyValue, 0, 3)
	if api := strings.TrimSpace(input.API); api != "" {
		attrs = append(attrs, attribute.String(llmtrace.TraceMetadataKeyAIGatewayAPI, api))
	}
	if requestID := strings.TrimSpace(input.RequestID); requestID != "" {
		attrs = append(attrs, attribute.String("aigateway.request.id", requestID))
	}
	if userID := strings.TrimSpace(input.UserID); userID != "" {
		attrs = append(attrs, attribute.String("user.id", userID))
	}
	traceCtx, span := otel.Tracer(requestPreflightTracerName).Start(ctx, "aigateway.request.preflight", oteltrace.WithAttributes(attrs...))
	return traceCtx, &preflightTrace{span: span}
}

func (t *preflightTrace) RecordError(err error, errorType string) {
	if t == nil || t.span == nil || !t.span.SpanContext().IsValid() || err == nil {
		return
	}
	t.span.RecordError(err)
	t.span.SetStatus(otelcodes.Error, err.Error())
	if errorType = strings.TrimSpace(errorType); errorType != "" {
		t.span.SetAttributes(
			attribute.String("error.type", errorType),
			attribute.String("error.category", "gateway_error"),
		)
	}
	t.End()
}

func (t *preflightTrace) SetTargetModel(requestModel string, target *resolvedModelTarget) {
	if t == nil || t.span == nil || !t.span.SpanContext().IsValid() || target == nil {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 4)
	if requestModel = strings.TrimSpace(requestModel); requestModel != "" {
		attrs = append(attrs, attribute.String("aigateway.request.model", requestModel))
	}
	if target.Model != nil {
		if modelID := strings.TrimSpace(target.Model.ID); modelID != "" {
			attrs = append(attrs, attribute.String(llmtrace.TraceMetadataKeyAIGatewayModelID, modelID))
		}
	}
	if provider := targetModelProvider(target); provider != "" {
		attrs = append(attrs, attribute.String("aigateway.target.provider", provider))
	}
	if resolvedModel := strings.TrimSpace(target.ModelName); resolvedModel != "" {
		attrs = append(attrs, attribute.String("aigateway.target.model", resolvedModel))
	}
	if fallbackCount := len(target.AttemptTargets); fallbackCount > 0 {
		attrs = append(attrs,
			attribute.Bool("aigateway.target.has_fallbacks", true),
			attribute.Int("aigateway.target.fallback_count", fallbackCount),
		)
	}
	if len(attrs) > 0 {
		t.span.SetAttributes(attrs...)
	}
}

func targetModelProvider(target *resolvedModelTarget) string {
	if target == nil {
		return ""
	}
	if provider := strings.TrimSpace(target.Upstream.Provider); provider != "" {
		return provider
	}
	if target.Model != nil {
		return strings.TrimSpace(target.Model.Provider)
	}
	return ""
}

func (t *preflightTrace) End() {
	if t == nil || t.span == nil {
		return
	}
	t.endOnce.Do(func() {
		t.span.End()
	})
}
