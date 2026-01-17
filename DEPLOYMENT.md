# Deployment Guide

## Current Deployment: GitHub Actions Continuous Loop

The bot runs **entirely on GitHub Actions** with **no external services or tokens required**. The workflow automatically restarts itself every ~5.5 hours, creating a continuous 24/7 operation.

### How It Works

1. Push code to GitHub (main branch)
2. GitHub Actions workflow starts
3. Bot runs for 5 hours 45 minutes
4. Before 6-hour timeout, workflow commits a trigger file
5. New workflow starts automatically
6. Loop continues forever → 24/7 operation!

### Initial Setup

**1. Add GitHub Secrets** (one-time):

Go to: https://github.com/Mehrrun/netblocks/settings/secrets/actions

Add these secrets:
- `TELEGRAM_BOT_TOKEN` = Your bot token
- `TELEGRAM_CHANNEL` = IranBlackoutMonitor

**2. Push to deploy**:
```bash
git push origin main
```

That's it! No external services, no API tokens, no authentication needed.

### How It Works Technically

```
Workflow 1: Runs bot for 5h 45m → Commits .trigger file → Exits
                                          ↓
Workflow 2: Triggered by .trigger commit → Runs bot for 5h 45m → Commits → Exits
                                                                    ↓
Workflow 3: ...and so on forever
```

**Key Features**:
- ✅ **No External Services**: Runs entirely on GitHub
- ✅ **No Extra Tokens**: Only needs GitHub's built-in `GITHUB_TOKEN`
- ✅ **24/7 Operation**: Auto-restarts before timeout
- ✅ **Free**: Uses GitHub Actions free tier
- ✅ **Auto-Deploy**: Push = instant deployment
- ✅ **No Setup**: Just add secrets and push

### Monitoring

**View bot status**:
- Go to: https://github.com/Mehrrun/netblocks/actions
- Click on the running workflow
- View logs in "Run for 5 hours 45 minutes" step

**Stop the bot**:
- Go to: https://github.com/Mehrrun/netblocks/actions
- Click on the running workflow
- Click "Cancel workflow"

**Restart the bot**:
```bash
# Make any commit or manually trigger
git commit --allow-empty -m "Restart bot"
git push
```

Or use the "Actions" tab → "Run Bot Continuously" → "Run workflow"

### Advantages

- ✅ **Zero Setup**: No external accounts or tokens
- ✅ **Free Forever**: GitHub Actions free tier includes 2000 minutes/month (bot uses ~1440 min/month)
- ✅ **Reliable**: Automatic restarts every 5.5 hours
- ✅ **Simple**: No complex infrastructure
- ✅ **Transparent**: All logs visible in GitHub Actions

### Limitations

- ⚠️ **Public repos only** for free tier (or use GitHub Pro)
- ⚠️ **2000 minutes/month limit** (sufficient for this bot)
- ⚠️ **5-10 second downtime** during restarts (minimal)

### Troubleshooting

**Bot not responding**:
- Check: https://github.com/Mehrrun/netblocks/actions
- View the latest workflow run
- Check logs for errors

**Workflow not restarting**:
- GitHub may rate-limit auto-commits
- Manually trigger: Actions → Run workflow

**Secrets not working**:
- Verify secrets are set: Settings → Secrets → Actions
- Check for typos in secret names

### Cost

**Completely FREE** if:
- Repository is public (unlimited minutes), OR
- You have GitHub Pro (3000 minutes/month)

For private repos on free tier:
- 2000 minutes/month free
- Bot uses ~1440 minutes/month (60 min/day × 24 days)
- **You have 560 minutes spare!**

## Alternative: Local Deployment

**Run locally with systemd** (VPS/server):

1. Copy bot binary to server
2. Create systemd service:

```bash
sudo nano /etc/systemd/system/netblocks-bot.service
```

```ini
[Unit]
Description=NetBlocks Telegram Bot
After=network.target

[Service]
Type=simple
User=yourusername
WorkingDirectory=/home/yourusername/netblocks
Environment="TELEGRAM_BOT_TOKEN=your_token"
Environment="TELEGRAM_CHANNEL=IranBlackoutMonitor"
ExecStart=/home/yourusername/netblocks/netblocks-telegram-bot
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

3. Start service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable netblocks-bot
sudo systemctl start netblocks-bot
```

## Local Development

**Run locally**:
```bash
export TELEGRAM_BOT_TOKEN=your_token
export TELEGRAM_CHANNEL=your_channel

go run ./cmd/telegram-bot
```

**Build and run**:
```bash
make telegram-bot
./netblocks-telegram-bot
```

## Production Checklist

- ✅ GitHub secrets configured (TELEGRAM_BOT_TOKEN, TELEGRAM_CHANNEL)
- ✅ .trigger file exists in repo
- ✅ Workflow file in .github/workflows/deploy.yml
- ✅ First workflow triggered (push to main)
- ✅ Bot responding to /status command
- ✅ Channel receiving periodic updates

## Support

- GitHub Actions: https://docs.github.com/en/actions
- Issues: https://github.com/Mehrrun/netblocks/issues
