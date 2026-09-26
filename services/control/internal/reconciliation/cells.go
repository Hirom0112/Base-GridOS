package reconciliation

import (
	"context"
	"sort"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CellDelivery struct {
	H3Cell             string
	Known              bool
	DeliveredMW        float64
	UncertainIntervals []report.UncertainInterval
}

func DeliveredByCell(ctx context.Context, pool *pgxpool.Pool, eventID string, deviceCells map[string]string, now time.Time, maxGap time.Duration) ([]CellDelivery, error) {
	if maxGap <= 0 {
		return nil, ErrMaxGapRequired
	}
	stored, err := loadEvent(ctx, pool, eventID)
	if err != nil {
		return nil, err
	}
	if _, supported := boundaryPower[stored.boundary]; !supported {
		return nil, ErrUnsupportedBoundary
	}
	measurement := Measurement{Begin: stored.begin, End: earliest(stored.end, now), MaxGap: maxGap}
	event := NewEvent(measurement)
	commandIDs, err := loadCommands(ctx, pool, event, eventID)
	if err != nil {
		return nil, err
	}
	if err = loadTelemetry(ctx, pool, event, stored.boundary); err != nil {
		return nil, err
	}
	uncertain, err := loadUncertain(ctx, pool, commandIDs)
	if err != nil {
		return nil, err
	}
	byCell := make(map[string]*Event)
	for _, command := range event.commands {
		cellID := deviceCells[command.deviceID]
		if cellID == "" {
			continue
		}
		cell := byCell[cellID]
		if cell == nil {
			cell = NewEvent(measurement)
			byCell[cellID] = cell
		}
		cell.Command(command.deviceID, command.Command)
		if command.acknowledged {
			cell.Acknowledge(command.ID)
		}
		if command.expiry.Before(command.ExpiresAt) {
			cell.Expire(command.ID, command.expiry)
		}
	}
	for cellID, cell := range byCell {
		for _, deviceID := range cell.deviceIDs() {
			for _, observation := range event.histories[deviceID].observations {
				cell.Observe(deviceID, observation)
			}
		}
		byCell[cellID] = cell
	}
	cells := make([]CellDelivery, 0, len(byCell))
	for cellID, event := range byCell {
		verification := event.Verify()
		delivery := summarize(verification, intervalsForCell(uncertain, deviceCells, cellID))
		cells = append(cells, CellDelivery{
			H3Cell: cellID, Known: verification.Measured > 0, DeliveredMW: delivery.DeliveredMW,
			UncertainIntervals: delivery.UncertainIntervals,
		})
	}
	sort.Slice(cells, func(left, right int) bool { return cells[left].H3Cell < cells[right].H3Cell })
	return cells, nil
}

func intervalsForCell(intervals []report.UncertainInterval, deviceCells map[string]string, cellID string) []report.UncertainInterval {
	filtered := make([]report.UncertainInterval, 0)
	for _, interval := range intervals {
		if deviceCells[interval.DeviceID] == cellID {
			filtered = append(filtered, interval)
		}
	}
	return filtered
}
