package vibe

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"time"
)

// Uses the application's existing OpenTelemetry provider/exporters. Prompts,
// outputs, cookies, keys and workspace/user IDs are never metric labels.
func traceCall(ctx context.Context, role Role, model, step string) (context.Context, func(error, *int64)) {
	started := time.Now()
	phase := attemptPhase(step, role)
	ctx, span := otel.Tracer("agentclash/vibe").Start(ctx, "vibe.model_call")
	span.SetAttributes(attribute.String("vibe.role", string(role)), attribute.String("gen_ai.request.model", model))
	return ctx, func(err error, cost *int64) {
		defer span.End()
		status := "ok"
		if err != nil {
			status = "error"
			span.SetStatus(codes.Error, "model invocation did not complete")
		}
		attrs := metric.WithAttributes(attribute.String("role", string(role)), attribute.String("stage", phase), attribute.String("status", status))
		meter := otel.Meter("agentclash/vibe")
		if duration, e := meter.Float64Histogram("vibe.stage.duration", metric.WithUnit("s")); e == nil {
			duration.Record(ctx, time.Since(started).Seconds(), attrs)
		}
		if calls, e := meter.Int64Counter("vibe.model.calls"); e == nil {
			calls.Add(ctx, 1, attrs)
		}
		if cost == nil {
			if unknown, e := meter.Int64Counter("vibe.model.uncertain_calls"); e == nil {
				unknown.Add(ctx, 1, attrs)
			}
		} else if spend, e := meter.Int64Counter("vibe.model.cost_nano_usd"); e == nil {
			spend.Add(ctx, *cost, attrs)
		}
	}
}
