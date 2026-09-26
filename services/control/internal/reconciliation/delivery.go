package reconciliation

import "time"

type Measurement struct {
	Begin  time.Time
	End    time.Time
	MaxGap time.Duration
}

type Gap struct {
	Begin time.Time
	End   time.Time
}

type Delivery struct {
	DeliveredKWh float64
	Measured     time.Duration
	Gaps         []Gap
}

func Integrate(measurement Measurement, observations []Telemetry) Delivery {
	var delivery Delivery
	cursor := measurement.Begin
	for index := 0; index+1 < len(observations); index++ {
		current, next := observations[index], observations[index+1]
		begin := latest(current.ObservedAt, measurement.Begin)
		end := earliest(next.ObservedAt, measurement.End)
		if !end.After(begin) {
			continue
		}
		if begin.After(cursor) {
			delivery.addGap(cursor, begin)
		}
		if next.ObservedAt.Sub(current.ObservedAt) > measurement.MaxGap {
			delivery.addGap(begin, end)
		} else {
			span := end.Sub(begin)
			delivery.DeliveredKWh += current.PowerKW * span.Hours()
			delivery.Measured += span
		}
		cursor = end
	}
	if cursor.Before(measurement.End) {
		delivery.addGap(cursor, measurement.End)
	}
	return delivery
}

func (delivery *Delivery) addGap(begin, end time.Time) {
	if last := len(delivery.Gaps) - 1; last >= 0 && delivery.Gaps[last].End.Equal(begin) {
		delivery.Gaps[last].End = end
		return
	}
	delivery.Gaps = append(delivery.Gaps, Gap{Begin: begin, End: end})
}

func latest(first, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}

func earliest(first, second time.Time) time.Time {
	if first.Before(second) {
		return first
	}
	return second
}
