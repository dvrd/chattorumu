#!/bin/bash

# Setup k6 Grafana Dashboard
# Imports the official k6 dashboard into Grafana

set -e

GRAFANA_URL="${GRAFANA_URL:-http://localhost:3100}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASS="${GRAFANA_PASS:-admin}"

echo "=================================================="
echo "k6 Grafana Dashboard Setup"
echo "=================================================="
echo ""
echo "Grafana URL: $GRAFANA_URL"
echo "Username: $GRAFANA_USER"
echo ""

# Check if Grafana is accessible
echo "Checking Grafana connectivity..."
if ! curl -s -o /dev/null -w "%{http_code}" "$GRAFANA_URL/api/health" | grep -q "200"; then
  echo "❌ Error: Cannot connect to Grafana at $GRAFANA_URL"
  echo "   Make sure Grafana is running: docker ps | grep grafana"
  exit 1
fi
echo "✅ Grafana is accessible"
echo ""

# Download k6 dashboard JSON from Grafana.com
echo "Downloading k6 dashboard (ID: 18595)..."
DASHBOARD_JSON=$(curl -s "https://grafana.com/api/dashboards/18595/revisions/1/download")

if [ -z "$DASHBOARD_JSON" ]; then
  echo "❌ Error: Failed to download dashboard"
  exit 1
fi
echo "✅ Dashboard downloaded"
echo ""

# Get Prometheus datasource UID
echo "Getting Prometheus datasource UID..."
DATASOURCE_UID=$(curl -s -u "$GRAFANA_USER:$GRAFANA_PASS" \
  "$GRAFANA_URL/api/datasources" | \
  grep -o '"uid":"[^"]*","name":"Prometheus"' | \
  sed 's/"uid":"//;s/","name":"Prometheus"//')

if [ -z "$DATASOURCE_UID" ]; then
  echo "❌ Error: Prometheus datasource not found"
  echo "   Make sure Prometheus datasource is configured in Grafana"
  exit 1
fi
echo "✅ Found Prometheus datasource: $DATASOURCE_UID"
echo ""

# Prepare dashboard with correct datasource
echo "Configuring dashboard..."
IMPORT_PAYLOAD=$(echo "$DASHBOARD_JSON" | jq --arg uid "$DATASOURCE_UID" '
{
  dashboard: .,
  overwrite: true,
  inputs: [
    {
      name: "DS_PROMETHEUS",
      type: "datasource",
      pluginId: "prometheus",
      value: $uid
    }
  ]
}')

# Import dashboard
echo "Importing dashboard into Grafana..."
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -u "$GRAFANA_USER:$GRAFANA_PASS" \
  -d "$IMPORT_PAYLOAD" \
  "$GRAFANA_URL/api/dashboards/import")

if echo "$RESPONSE" | grep -q '"status":"success"'; then
  DASHBOARD_URL=$(echo "$RESPONSE" | jq -r '.importedUrl')
  echo "✅ Dashboard imported successfully!"
  echo ""
  echo "=================================================="
  echo "🎉 Setup Complete!"
  echo "=================================================="
  echo ""
  echo "Dashboard URL:"
  echo "  $GRAFANA_URL$DASHBOARD_URL"
  echo ""
  echo "Next steps:"
  echo "  1. Run a load test: task load:run:chat"
  echo "  2. Open the dashboard above"
  echo "  3. Watch metrics in real-time"
  echo ""
else
  echo "❌ Error importing dashboard:"
  echo "$RESPONSE" | jq '.'
  exit 1
fi
