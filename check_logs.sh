#!/bin/bash
echo "🔍 Fetching latest workflow logs..."
echo ""

RUN_ID=$(gh run list --limit 1 --json databaseId --jq '.[0].databaseId')
echo "Run ID: $RUN_ID"
echo ""

# Get job ID
JOB_ID=$(gh api "repos/Mehrrun/netblocks/actions/runs/$RUN_ID/jobs" | jq -r '.jobs[0].id')
echo "Job ID: $JOB_ID"
echo ""

# Fetch logs
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 FULL STARTUP LOGS:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
curl -sL -H "Authorization: Bearer $(gh auth token)" \
  "https://api.github.com/repos/Mehrrun/netblocks/actions/jobs/$JOB_ID/logs" | \
  strings | grep "2026/01" | head -100

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔑 CLOUDFLARE-RELATED LOGS:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
curl -sL -H "Authorization: Bearer $(gh auth token)" \
  "https://api.github.com/repos/Mehrrun/netblocks/actions/jobs/$JOB_ID/logs" | \
  strings | grep -iE "(cloudflare|traffic|chart|📡|🔑|📊)" | head -30

echo ""
echo "✅ Done!"
