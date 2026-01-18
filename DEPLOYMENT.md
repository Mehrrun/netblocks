# Deployment Guide

## Current Deployment: GitHub Actions (Forever Running)

The bot runs **24/7 on GitHub Actions** with **zero downtime** and **no external services required**. The workflow stays "Running" (yellow status) indefinitely, which is the expected behavior.

### How It Works

The GitHub Actions workflow:
1. Builds the bot binary
2. Starts the bot in background with environment variables
3. Enters an infinite monitoring loop
4. Checks bot health every 5 minutes
5. Shows logs every 50 minutes
6. **Never completes** - runs forever!

### Architecture

```
┌─────────────────────────────────────────────┐
│  GitHub Actions Workflow (runs forever)     │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │  Bot Process (background)           │   │
│  │  - BGP monitoring (RIS Live)        │   │
│  │  - DNS monitoring                   │   │
│  │  - Telegram bot (long polling)      │   │
│  │  - Periodic updates to channel      │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │  Monitoring Loop (foreground)       │   │
│  │  - Check bot alive every 5 min      │   │
│  │  - Show logs every 50 min           │   │
│  │  - Runs forever (never exits)       │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  Status: Running (yellow) ← Forever!       │
└─────────────────────────────────────────────┘
```

## Initial Setup (One-time)

### 1. Add GitHub Secrets

Go to: https://github.com/Mehrrun/netblocks/settings/secrets/actions

Add these secrets:
- `TELEGRAM_BOT_TOKEN` = Your Telegram bot token
- `TELEGRAM_CHANNEL` = Your channel username (e.g., `IranBlackoutMonitor`)

### 2. Deploy

Simply push to the main branch or trigger manually:

```bash
git push origin main
```

Or use the GitHub Actions UI:
- Go to: https://github.com/Mehrrun/netblocks/actions
- Click "Run Bot 24/7"
- Click "Run workflow"

**That's it!** The bot will start and run forever.

## Managing the Bot

### View Status

Go to: https://github.com/Mehrrun/netblocks/actions

You should see:
- A workflow with yellow "Running" status (this is correct!)
- Click on it to view real-time logs
- Every 5 minutes: health check log
- Every 50 minutes: recent bot activity

### View Logs

```bash
# Click on the running workflow
# Then click "Monitor bot forever" step
# You'll see live output from the bot
```

### Stop Bot

To stop the bot:
1. Go to: https://github.com/Mehrrun/netblocks/actions
2. Click on the running workflow
3. Click "Cancel workflow" button (top right)

### Restart Bot

To restart the bot:
1. **Cancel** the current running workflow (if any)
2. Then either:
   - Push a new commit: `git push origin main`
   - Or manually trigger: Actions → Run Bot 24/7 → Run workflow

### Update Bot Code

To deploy code changes:
1. Make your changes locally
2. Commit: `git commit -am "Your changes"`
3. **Cancel the running workflow first** (important!)
4. Push: `git push origin main`
5. New workflow will start automatically with updated code

## Advantages

- ✅ **Zero Downtime**: Bot runs continuously 24/7
- ✅ **No External Services**: No Fly.io, Railway, or VPS needed
- ✅ **No Extra Tokens**: Only GitHub repository secrets
- ✅ **Free**: Uses GitHub Actions (free tier sufficient for public repos)
- ✅ **Real-time Logs**: View bot activity in workflow logs
- ✅ **Simple Management**: Cancel/restart via GitHub UI
- ✅ **Automatic Deployment**: Push to deploy

## Expected Behavior

### Normal Status

✅ **Workflow shows as "Running" (yellow status)** - This is correct!

The workflow is designed to never complete. It will stay yellow/running forever while the bot operates. This is **not an error**.

### When to Worry

❌ **Workflow shows as "Failed" (red status)** - Bot crashed

If the workflow fails (red), check the logs:
1. Go to the failed workflow run
2. Click on "Monitor bot forever" step
3. Read the error message and full logs
4. Fix the issue in your code
5. Push changes (workflow will restart)

## Cost

**Completely FREE** if:
- Repository is public (unlimited minutes)
- Or you have GitHub Pro (3000 minutes/month)

For private repos on free tier:
- 2000 minutes/month free
- Bot uses all available minutes (runs continuously)
- Consider making repo public or upgrading to Pro

## Troubleshooting

### Bot not responding to commands

1. Check workflow is running: https://github.com/Mehrrun/netblocks/actions
2. View logs in "Monitor bot forever" step
3. Look for errors or warnings
4. Verify bot token is correct in GitHub secrets

### Channel not receiving updates

1. Verify `TELEGRAM_CHANNEL` secret is set correctly
2. Check bot is admin in the channel
3. Look for "channel" related errors in logs
4. Test with `/status` command in bot DM first

### Workflow keeps failing

1. Read the error message in workflow logs
2. Common issues:
   - Invalid bot token → Update `TELEGRAM_BOT_TOKEN` secret
   - Missing secrets → Add all required secrets
   - Code errors → Check error message and fix code
3. After fixing, push changes to restart

### Want to run locally for testing

```bash
# Set environment variables
export TELEGRAM_BOT_TOKEN=your_token
export TELEGRAM_CHANNEL=your_channel

# Build and run
make telegram-bot
./netblocks-telegram-bot
```

## Workflow Details

### Timeout

The workflow has a timeout of 525,600 minutes (1 year, the maximum allowed by GitHub Actions). In practice:
- Public repos: No practical limit
- Private repos: Limited by available minutes

### Health Checks

Every 5 minutes, the workflow:
1. Checks if bot process is still alive
2. Logs timestamp and check number
3. If bot dies: Shows full logs and exits with error

Every 50 minutes, the workflow:
- Shows last 30 lines of bot activity
- Helps you verify bot is working correctly

### Logs

All bot output is saved to `bot.log` file in the workflow:
- Startup messages
- BGP monitoring events
- DNS check results
- Telegram messages sent
- Errors and warnings

## Alternative: Local Deployment (VPS/Server)

If you prefer running on your own server:

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
Environment="TELEGRAM_CHANNEL=your_channel"
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
sudo systemctl status netblocks-bot
```

## Support

- GitHub Issues: https://github.com/Mehrrun/netblocks/issues
- GitHub Actions Docs: https://docs.github.com/en/actions
- Telegram Bot API: https://core.telegram.org/bots/api
