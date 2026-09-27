package dispatch

import (
	"context"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const PruneTelemetryActivity = "PruneTelemetry"
const RiskOverridesActivity = "EvaluateRiskOverrides"

type TelemetryMaintenanceActivities struct {
	Store *storage.TelemetryStore
	Now   func() time.Time
}

type RiskOverrideActivities struct {
	Bridge *policy.RiskBridge
}

func (activities *RiskOverrideActivities) EvaluateRiskOverrides(ctx context.Context, at time.Time) error {
	return activities.Bridge.Evaluate(ctx, at)
}

func (activities *TelemetryMaintenanceActivities) PruneTelemetry(ctx context.Context) error {
	_, err := activities.Store.Prune(ctx, activities.Now())
	return err
}

func TelemetryMaintenance(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 10 * time.Second,
			MaximumAttempts: 1,
		},
	})
	return workflow.ExecuteActivity(ctx, PruneTelemetryActivity).Get(ctx, nil)
}

func RiskOverrides(ctx workflow.Context) error {
	at := workflow.Now(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 10 * time.Second,
			MaximumAttempts: 1,
		},
	})
	return workflow.ExecuteActivity(ctx, RiskOverridesActivity, at).Get(ctx, nil)
}
