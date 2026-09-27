package dispatch

import (
	"context"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const PruneTelemetryActivity = "PruneTelemetry"

type TelemetryMaintenanceActivities struct {
	Store *storage.TelemetryStore
	Now   func() time.Time
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
