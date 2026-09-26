package dispatch

import "context"

func FreezeInputs(context.Context, Input) error { return nil }

func RequestPlan(context.Context, Input) error { return nil }

func ValidatePlan(context.Context, Input) error { return nil }

func PersistIntents(context.Context, Input) error { return nil }

func PublishCommands(context.Context, Input) error { return nil }

func TrackAcknowledgements(context.Context, Input) error { return nil }

func VerifyDelivery(context.Context, Input) error { return nil }

func EndEvent(context.Context, Input) error { return nil }

func ReconcileLateMessages(context.Context, Input) error { return nil }

func ProduceReport(context.Context, Input) error { return nil }

func IssueEmergencyStop(context.Context, EmergencyCommand) error { return nil }
