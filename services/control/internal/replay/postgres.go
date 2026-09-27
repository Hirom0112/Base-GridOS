package replay

import (
	"context"
	"errors"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type PostgresSource struct {
	Pool *pgxpool.Pool
}

func (source PostgresSource) Load(ctx context.Context, manifest Manifest) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	if source.Pool == nil {
		return nil, nil, errors.New("replay database is required")
	}
	var inputJSON, planJSON []byte
	err := source.Pool.QueryRow(ctx, `SELECT input.inputs, plan.plan
		FROM plan_versions AS plan
		JOIN input_snapshots AS input ON input.snapshot_id = plan.input_snapshot_id
		JOIN eligibility_snapshots AS eligibility ON eligibility.snapshot_id = plan.eligibility_snapshot_id
		WHERE plan.event_id = $1 AND plan.input_snapshot_id = $2 AND plan.eligibility_snapshot_id = $3
		AND input.event_id = $1 AND eligibility.event_id = $1
		ORDER BY plan.version LIMIT 1`, manifest.EventID, manifest.InputSnapshotID, manifest.EligibilitySnapshotID).Scan(&inputJSON, &planJSON)
	if err != nil {
		return nil, nil, err
	}
	request := new(gridosv1.OptimizationRequest)
	if err = protojson.Unmarshal(inputJSON, request); err != nil {
		return nil, nil, err
	}
	plan := new(gridosv1.DispatchPlan)
	if err = protojson.Unmarshal(planJSON, plan); err != nil {
		return nil, nil, err
	}
	return request, plan, nil
}
