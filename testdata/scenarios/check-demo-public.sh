set -eu
demo_directory=$(mktemp -d)
trap 'rm -rf "$demo_directory"' EXIT
GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml make --no-print-directory demo-public GRIDOS_DEMO_DIR="$demo_directory"
test -f "$demo_directory/public/ercot-prices/dam-spp-week.csv"
test -f "$demo_directory/public/weather/austin_forecast.json"
test "$(head -n 1 "$demo_directory/public/weather/PROVENANCE.md")" = 'Provenance: SIMULATED'
jq -e '.features | length == 1 and .[0].properties.geocode.UGC == ["TXZ192"] and .[0].properties.severity == "Severe" and .[0].properties.expires == "2100-01-01T00:00:00Z"' "$demo_directory/public/weather/austin_alerts.json" >/dev/null
for city in dallas houston san_antonio; do
  jq -e '.features | length == 0' "$demo_directory/public/weather/${city}_alerts.json" >/dev/null
done
test "$(make -n demo GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml GRIDOS_DEMO_DIR="$demo_directory" | grep -F -c "GRIDOS_PUBLIC_CONTEXT_DIR='$demo_directory/public'")" -eq 2
printf 'demo public weather assembly: PASS\n'
