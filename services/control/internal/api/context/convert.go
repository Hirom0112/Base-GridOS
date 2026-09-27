package context

import (
	"fmt"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	publiccontext "github.com/Hirom0112/Base-GridOS/services/control/internal/context"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var provenances = map[string]gridosv1.DataProvenance{
	"CONFIRMED_PUBLIC": gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC,
	"DERIVED":          gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED,
	"SIMULATED":        gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED,
}

func contextSource(source publiccontext.Source) (*gridosv1.ContextSource, error) {
	provenance, found := provenances[source.Provenance]
	if !found {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unsupported public context provenance %q", source.Provenance))
	}
	return &gridosv1.ContextSource{Provenance: provenance, AsOf: timestamppb.New(source.AsOf), Freshness: durationpb.New(source.Age)}, nil
}

func marketPrice(price publiccontext.Price) (*gridosv1.ContextMarketPrice, error) {
	source, err := contextSource(price.Source)
	if err != nil {
		return nil, err
	}
	return &gridosv1.ContextMarketPrice{
		IntervalEnd: timestamppb.New(price.At), SettlementPoint: price.SettlementPoint,
		UsdPerMwh: price.USDPerMWh, Source: source,
	}, nil
}

func settlementPrices(prices []publiccontext.Price, settlementPoint string) ([]*gridosv1.ContextMarketPrice, error) {
	var matched []*gridosv1.ContextMarketPrice
	for _, price := range prices {
		if price.SettlementPoint != settlementPoint {
			continue
		}
		converted, err := marketPrice(price)
		if err != nil {
			return nil, err
		}
		matched = append(matched, converted)
	}
	return matched, nil
}
