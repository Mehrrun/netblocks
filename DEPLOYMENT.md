# Deployment Guide

## Current Deployment: Fly.io (Recommended)

The bot is deployed to **Fly.io** for 24/7 operation. GitHub Actions handles automatic deployment when you push to the main branch.

### How It Works

1. Push code to GitHub (main branch)
2. GitHub Actions builds the Docker image
3. Deploys to Fly.io automatically
4. Bot runs continuously on Fly.io (never stops)
5. Auto-restarts if it crashes

### Initial Setup (One-time)

1. **Install Fly CLI locally**:
   ```bash
   curl -L https://fly.io/install.sh | sh
   ```

2. **Login to Fly.io**:
   ```bash
   fly auth login
   ```

3. **Create the app** (first time only):
   ```bash
   fly launch --no-deploy
   # Choose app name: netblocks-bot
   # Choose region: Amsterdam (ams) or closest to you
   ```

4. **Set secrets on Fly.io**:
   ```bash
   fly secrets set TELEGRAM_BOT_TOKEN=your_token_here
   fly secrets set TELEGRAM_CHANNEL=IranBlackoutMonitor
   ```

5. **Get Fly API token for GitHub Actions**:
   ```bash
   fly tokens create deploy
   ```
   Copy the token, then add it to GitHub:
   - Go to your repo → Settings → Secrets and variables → Actions
   - Add secret: `FLY_API_TOKEN` = (paste the token)

6. **Deploy**:
   ```bash
   git push origin main
   ```
   GitHub Actions will automatically deploy to Fly.io!

### Managing the Bot

**View status**:
```bash
fly status --app netblocks-bot
```

**View logs**:
```bash
fly logs --app netblocks-bot
```

**Restart bot**:
```bash
fly apps restart netblocks-bot
```

**Scale resources** (if needed):
```bash
fly scale memory 512 --app netblocks-bot
```

**SSH into the machine**:
```bash
fly ssh console --app netblocks-bot
```

### Advantages

- ✅ **24/7 Uptime**: Runs continuously, no timeouts
- ✅ **Auto-Deploy**: Push to GitHub = automatic deployment
- ✅ **Auto-Restart**: Crashes are automatically recovered
- ✅ **Free Tier**: Sufficient for this bot
- ✅ **Global CDN**: Fast worldwide
- ✅ **Easy Logs**: `fly logs` command

### Cost

**Free tier includes**:
- Up to 3 shared-cpu-1x VMs
- 256MB RAM per VM
- 160GB outbound data transfer

This bot uses minimal resources and fits comfortably in the free tier.

## Alternative: GitHub Actions with systemd (Development Only)

For testing purposes, you can run with systemd on GitHub Actions, but it will timeout after 6 hours.

See the `netblocks-bot.service` file for systemd configuration.

## Local Development

**Run locally**:
```bash
# Set environment variables
export TELEGRAM_BOT_TOKEN=your_token
export TELEGRAM_CHANNEL=your_channel

# Build and run
go build -o netblocks-telegram-bot ./cmd/telegram-bot
./netblocks-telegram-bot
```

**Run with Docker**:
```bash
# Build image
docker build -t netblocks-bot .

# Run container
docker run -e TELEGRAM_BOT_TOKEN=your_token \
           -e TELEGRAM_CHANNEL=your_channel \
           netblocks-bot
```

## Monitoring

**Fly.io Dashboard**: https://fly.io/dashboard
- View app status
- Check metrics (CPU, memory, network)
- View deployment history

**Telegram**: 
- Send `/status` to your bot
- Bot sends updates to your channel every 10 minutes

**Logs**:
```bash
# Follow logs in real-time
fly logs --app netblocks-bot

# Show last 100 lines
fly logs --app netblocks-bot -n 100
```

## Troubleshooting

**Bot not responding**:
```bash
# Check if app is running
fly status --app netblocks-bot

# View recent logs
fly logs --app netblocks-bot

# Restart if needed
fly apps restart netblocks-bot
```

**Deployment fails**:
```bash
# Check deployment logs in GitHub Actions
# Or deploy manually:
fly deploy
```

**Update secrets**:
```bash
fly secrets set TELEGRAM_BOT_TOKEN=new_token --app netblocks-bot
```

**View all secrets** (names only, not values):
```bash
fly secrets list --app netblocks-bot
```

## Production Checklist

- ✅ Fly.io app created
- ✅ Secrets set (TELEGRAM_BOT_TOKEN, TELEGRAM_CHANNEL)
- ✅ FLY_API_TOKEN added to GitHub secrets
- ✅ GitHub Actions workflow configured
- ✅ Bot deployed and running
- ✅ Bot responding to Telegram commands
- ✅ Channel receiving updates

## Support

- Fly.io Docs: https://fly.io/docs/
- Fly.io Community: https://community.fly.io/
