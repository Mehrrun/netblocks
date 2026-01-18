#!/bin/bash
echo "Testing Cloudflare Radar API with your token..."
echo ""

# Get token from GitHub secret
TOKEN=$(gh secret get CLOUDFLARE_TOKEN 2>/dev/null)

if [ -z "$TOKEN" ]; then
    echo "❌ CLOUDFLARE_TOKEN secret is empty!"
    exit 1
fi

echo "✅ Token loaded (length: ${#TOKEN} chars)"
echo ""
echo "📡 Testing API call..."
echo ""

# Test the API
RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  -H "User-Agent: NetBlocks-Monitor/1.0" \
  "https://api.cloudflare.com/client/v4/radar/http/timeseries_groups/bandwidth?location=IR&dateRange=24h&aggInterval=1h")

HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v "HTTP_STATUS:")

echo "📊 HTTP Status: $HTTP_STATUS"
echo ""

if [ "$HTTP_STATUS" = "200" ]; then
    echo "✅ SUCCESS! API call worked!"
    echo ""
    echo "Response preview:"
    echo "$BODY" | jq -r '.success, .result.serie_0.timestamps[0], (.result.serie_0.values | length)' 2>/dev/null || echo "$BODY" | head -20
else
    echo "❌ FAILED! API returned error:"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
fi
