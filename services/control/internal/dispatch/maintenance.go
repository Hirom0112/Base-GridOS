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
const PostPeriodicRewardsActivity = "PostPeriodicRewards"
const PostLateEventRewardsActivity = "PostLateEventRewards"
const RiskOverridesActivity = "EvaluateRiskOverrides"
const RiskAnomaliesActivity = "EvaluateRiskAnomalies"

type TelemetryMaintenanceActivities struct {
	Store   *storage.TelemetryStore
	Rewards *policy.Store
	Now     func() time.Time
}

type RiskOverrideActivities struct {
	Bridge *policy.RiskBridge
}

func (activities *RiskOverrideActivities) EvaluateRiskOverrides(ctx context.Context, at time.Time) error {
	return activities.Bridge.Evaluate(ctx, at)
}

func (activities *RiskOverrideActivities) EvaluateRiskAnomalies(ctx context.Context, at time.Time) error {
	return activities.Bridge.EvaluateAnomalies(ctx, at)
}

func (activities *TelemetryMaintenanceActivities) PruneTelemetry(ctx context.Context) error {
	_, err := activities.Store.Prune(ctx, activities.Now())
	return err
}

func (activities *TelemetryMaintenanceActivities) PostPeriodicRewards(ctx context.Context, at time.Time) error {
	_, err := activities.Rewards.PostPeriodicRewards(ctx, at)
	return err
}

func (activities *TelemetryMaintenanceActivities) PostLateEventRewards(ctx context.Context, at time.Time) error {
	_, err := activities.Rewards.PostLateEventRewards(ctx, at)
	return err
}

func TelemetryMaintenance(ctx workflow.Context) error {
	at := workflow.Now(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 10 * time.Second,
			MaximumAttempts: 1,
		},
	})
	if err := workflow.ExecuteActivity(ctx, PruneTelemetryActivity).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, PostPeriodicRewardsActivity, at).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, PostLateEventRewardsActivity, at).Get(ctx, nil)
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
	if err := workflow.ExecuteActivity(ctx, RiskOverridesActivity, at).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, RiskAnomaliesActivity, at).Get(ctx, nil)
}
