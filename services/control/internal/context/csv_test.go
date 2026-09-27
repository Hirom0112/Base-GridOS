package context

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRealTimeKeepsSettlementPriceOverEnergyWeightedZone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtm.csv")
	rows := "Delivery Date,Delivery Hour,Delivery Interval,Repeated Hour Flag,Settlement Point Name,Settlement Point Type,Settlement Point Price\n" +
		"01/01/2025,1,1,N,HB_HOUSTON,HU,17.9\n" +
		"01/01/2025,1,1,N,LZ_AEN,LZ,20.5\n" +
		"01/01/2025,1,1,N,LZ_AEN,LZEW,21.75\n"
	if err := os.WriteFile(path, []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}
	prices, err := loadRealTime(path, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]float64{}
	for _, price := range prices {
		key := price.SettlementPoint + price.At.String()
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate real-time price for %s at %s", price.SettlementPoint, price.At)
		}
		seen[key] = price.USDPerMWh
	}
	if len(prices) != 2 || seen["HB_HOUSTON"+prices[0].At.String()] != 17.9 || seen["LZ_AEN"+prices[0].At.String()] != 20.5 {
		t.Fatalf("real-time prices=%+v", prices)
	}
}
