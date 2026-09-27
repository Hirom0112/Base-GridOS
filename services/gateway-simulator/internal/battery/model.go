package battery

import (
	"errors"
	"fmt"
	"math"
	"time"
)

type OperatingState string

const (
	OnGrid                    OperatingState = "ON_GRID"
	OffGridOutage             OperatingState = "OFF_GRID_OUTAGE"
	OffGridNoHomePower        OperatingState = "OFF_GRID_NO_HOME_POWER"
	OffGridOvercurrent        OperatingState = "OFF_GRID_OVERCURRENT"
	OffGridOvercurrentStandby OperatingState = "OFF_GRID_OVERCURRENT_STANDBY"
	TelemetryUnavailable      OperatingState = "TELEMETRY_UNAVAILABLE"
)

type Parameters struct {
	UsableEnergyKWh     float64
	ReservePercent      float64
	HardwareFloorKWh    float64
	MaxChargeKW         float64
	MaxDischargeKW      float64
	ChargeEfficiency    float64
	DischargeEfficiency float64
	RampLimitKWPerHour  float64
	OvercurrentLimitKW  float64
	TemperatureDerate   func(float64) float64
}

type Input struct {
	ChargeKW               float64
	DischargeKW            float64
	Duration               time.Duration
	CurrentLoadKW          float64
	TemperatureCelsius     float64
	GridOutage             bool
	HomePowerUnavailable   bool
	ProtectionStandby      bool
	TelemetryUnavailable   bool
	GridConnectionVerified bool
}

type Result struct {
	EnergyKWh               float64
	OperatingState          OperatingState
	GridServiceCapacityKW   float64
	BackupHoursCurrentUsage float64
	BackupHours750W         float64
}

type Model struct {
	parameters Parameters
	energyKWh  float64
	lastPower  float64
	state      OperatingState
}

func New(parameters Parameters, energyKWh float64, state OperatingState) (*Model, error) {
	if err := validateParameters(parameters, energyKWh); err != nil {
		return nil, err
	}
	if !validState(state) {
		return nil, fmt.Errorf("invalid operating state %q", state)
	}
	return &Model{parameters: parameters, energyKWh: energyKWh, state: state}, nil
}

func (model *Model) EnergyKWh() float64 {
	return model.energyKWh
}

func validateParameters(parameters Parameters, energyKWh float64) error {
	values := []float64{parameters.UsableEnergyKWh, parameters.HardwareFloorKWh, parameters.MaxChargeKW, parameters.MaxDischargeKW, parameters.ChargeEfficiency, parameters.DischargeEfficiency, parameters.RampLimitKWPerHour, parameters.OvercurrentLimitKW, energyKWh}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("parameters must be finite")
		}
	}
	if parameters.UsableEnergyKWh <= 0 || parameters.HardwareFloorKWh < 0 || parameters.HardwareFloorKWh > energyKWh || energyKWh > parameters.UsableEnergyKWh {
		return errors.New("invalid energy bounds")
	}
	if parameters.MaxChargeKW < 0 || parameters.MaxDischargeKW < 0 || parameters.RampLimitKWPerHour < 0 || parameters.OvercurrentLimitKW < 0 {
		return errors.New("invalid power bounds")
	}
	if parameters.ChargeEfficiency <= 0 || parameters.ChargeEfficiency > 1 || parameters.DischargeEfficiency <= 0 || parameters.DischargeEfficiency > 1 {
		return errors.New("invalid efficiencies")
	}
	return nil
}

func validState(state OperatingState) bool {
	switch state {
	case OnGrid, OffGridOutage, OffGridNoHomePower, OffGridOvercurrent, OffGridOvercurrentStandby, TelemetryUnavailable:
		return true
	default:
		return false
	}
}

func (model *Model) Step(input Input) (Result, error) {
	if err := model.validateInput(input); err != nil {
		return Result{}, err
	}
	hours := input.Duration.Hours()
	nextEnergy := model.energyKWh + model.parameters.ChargeEfficiency*input.ChargeKW*hours - input.DischargeKW*hours/model.parameters.DischargeEfficiency
	if nextEnergy < model.parameters.HardwareFloorKWh || nextEnergy > model.parameters.UsableEnergyKWh {
		return Result{}, errors.New("energy bound exceeded")
	}
	model.energyKWh = nextEnergy
	model.lastPower = input.DischargeKW - input.ChargeKW
	model.state = model.nextState(input)
	return model.result(input.CurrentLoadKW), nil
}

func (model *Model) validateInput(input Input) error {
	if input.Duration <= 0 || input.ChargeKW < 0 || input.DischargeKW < 0 || input.CurrentLoadKW < 0 {
		return errors.New("invalid step input")
	}
	if input.ChargeKW > 0 && input.DischargeKW > 0 {
		return errors.New("charge and discharge are mutually exclusive")
	}
	derate := 1.0
	if model.parameters.TemperatureDerate != nil {
		derate = model.parameters.TemperatureDerate(input.TemperatureCelsius)
	}
	if derate < 0 || derate > 1 || input.ChargeKW > model.parameters.MaxChargeKW*derate || input.DischargeKW > model.parameters.MaxDischargeKW*derate {
		return errors.New("power bound exceeded")
	}
	nextPower := input.DischargeKW - input.ChargeKW
	if model.parameters.RampLimitKWPerHour > 0 && math.Abs(nextPower-model.lastPower) > model.parameters.RampLimitKWPerHour*input.Duration.Hours() {
		return errors.New("ramp limit exceeded")
	}
	return nil
}

func (model *Model) nextState(input Input) OperatingState {
	if input.TelemetryUnavailable {
		return TelemetryUnavailable
	}
	if input.GridConnectionVerified {
		return OnGrid
	}
	if model.state == OnGrid && input.GridOutage {
		return OffGridOutage
	}
	if model.state == OffGridOutage && input.HomePowerUnavailable {
		return OffGridNoHomePower
	}
	if model.state == OffGridOutage && input.CurrentLoadKW > model.parameters.OvercurrentLimitKW {
		return OffGridOvercurrent
	}
	if model.state == OffGridOvercurrent && input.ProtectionStandby {
		return OffGridOvercurrentStandby
	}
	return model.state
}

func (model *Model) result(currentLoadKW float64) Result {
	availableAC := max(0, model.energyKWh-model.parameters.HardwareFloorKWh) * model.parameters.DischargeEfficiency
	currentHours := 0.0
	if currentLoadKW > 0 {
		currentHours = availableAC / currentLoadKW
	}
	capacity := min(model.parameters.MaxDischargeKW, availableAC)
	if model.state != OnGrid {
		capacity = 0
	}
	return Result{
		EnergyKWh:               model.energyKWh,
		OperatingState:          model.state,
		GridServiceCapacityKW:   capacity,
		BackupHoursCurrentUsage: currentHours,
		BackupHours750W:         availableAC / 0.75,
	}
}
