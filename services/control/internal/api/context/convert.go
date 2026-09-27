package context

import (
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	publiccontext "github.com/Hirom0112/Base-GridOS/services/control/internal/context"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func contextSource(source publiccontext.Source) *gridosv1.ContextSource {
	var provenance gridosv1.DataProvenance
	switch source.Provenance {
	case "CONFIRMED_PUBLIC":
		provenance = gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC
	case "DERIVED":
		provenance = gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED
	default:
		panic("unvalidated public context provenance")
	}
	return &gridosv1.ContextSource{Provenance: provenance, AsOf: timestamppb.New(source.AsOf), Freshness: durationpb.New(source.Age)}
}

func marketPrice(price publiccontext.Price) *gridosv1.ContextMarketPrice {
	return &gridosv1.ContextMarketPrice{
		IntervalEnd: timestamppb.New(price.At), SettlementPoint: price.SettlementPoint,
		UsdPerMwh: price.USDPerMWh, Source: contextSource(price.Source),
	}
}
