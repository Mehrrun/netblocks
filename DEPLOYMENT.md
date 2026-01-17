# Deployment Guide

## Current Deployment: GitHub Actions

The bot is currently configured to run on GitHub Actions. However, **GitHub Actions is not ideal for long-running services** due to:

- ⏱️ **6-hour timeout limit** on free tier
- 💰 **Usage limits** on free tier
- 🔄 **Workflow cancellation** when repository is inactive

### How It Works

1. The workflow builds the bot binary
2. Runs the bot in the background using `nohup`
3. Verifies the bot started successfully
4. Keeps the workflow alive to maintain the bot service
5. Monitors bot health every 60 seconds

### Limitations

- The workflow will timeout after 6 hours
- If the workflow is cancelled, the bot stops
- Not suitable for 24/7 production use

## Recommended Production Deployment Options

### Option 1: Railway.app (Recommended)

Railway is perfect for long-running services:

1. **Sign up**: https://railway.app
2. **Connect GitHub**: Link your repository
3. **Deploy**: Railway auto-detects Go projects
4. **Set Environment Variables**:
   - `TELEGRAM_BOT_TOKEN`: Your bot token
   - `TELEGRAM_CHANNEL`: Your channel username
5. **Deploy**: Railway will build and run automatically

**Cost**: Free tier available, then pay-as-you-go

### Option 2: Render.com

1. **Sign up**: https://render.com
2. **New Web Service**: Connect GitHub repo
3. **Build Command**: `go build -o netblocks-telegram-bot ./cmd/telegram-bot`
4. **Start Command**: `./netblocks-telegram-bot`
5. **Environment Variables**: Add `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHANNEL`

**Cost**: Free tier available

### Option 3: Fly.io

1. **Install Fly CLI**: `curl -L https://fly.io/install.sh | sh`
2. **Login**: `fly auth login`
3. **Launch**: `fly launch` (in project directory)
4. **Set Secrets**: 
   ```bash
   fly secrets set TELEGRAM_BOT_TOKEN=your_token
   fly secrets set TELEGRAM_CHANNEL=your_channel
   ```
5. **Deploy**: `fly deploy`

**Cost**: Free tier available

### Option 4: VPS (DigitalOcean, Linode, etc.)

For full control:

1. **Create VPS**: Ubuntu 22.04 LTS
2. **Install Go**: 
   ```bash
   wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin
   ```
3. **Clone Repository**:
   ```bash
   git clone https://github.com/mehrrun/netblocks.git
   cd netblocks
   ```
4. **Build**:
   ```bash
   go build -o netblocks-telegram-bot ./cmd/telegram-bot
   ```
5. **Create systemd Service** (`/etc/systemd/system/netblocks-bot.service`):
   ```ini
   [Unit]
   Description=NetBlocks Telegram Bot
   After=network.target

   [Service]
   Type=simple
   User=your-user
   WorkingDirectory=/path/to/netblocks
   Environment="TELEGRAM_BOT_TOKEN=your_token"
   Environment="TELEGRAM_CHANNEL=your_channel"
   ExecStart=/path/to/netblocks/netblocks-telegram-bot
   Restart=always
   RestartSec=10

   [Install]
   WantedBy=multi-user.target
   ```
6. **Enable and Start**:
   ```bash
   sudo systemctl enable netblocks-bot
   sudo systemctl start netblocks-bot
   sudo systemctl status netblocks-bot
   ```

**Cost**: $5-10/month for basic VPS

## Environment Variables

All deployment methods require these environment variables:

- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token
- `TELEGRAM_CHANNEL`: Your Telegram channel username (e.g., `IranBlackoutMonitor`)

Or create a `config.json` file with:
```json
{
  "telegram_token": "your_token",
  "telegram_channel": "your_channel",
  "interval": "10m"
}
```

## Monitoring

Check bot status:

- **GitHub Actions**: View workflow logs in Actions tab
- **Railway/Render**: View logs in dashboard
- **Fly.io**: `fly logs`
- **VPS**: `sudo systemctl status netblocks-bot` or `journalctl -u netblocks-bot -f`

## Troubleshooting

### Bot Not Starting

1. Check logs for errors
2. Verify environment variables are set
3. Ensure bot token is valid
4. Check network connectivity

### Bot Stops Unexpectedly

1. Check logs for error messages
2. Verify bot has proper permissions
3. Check system resources (memory, CPU)
4. Ensure no rate limiting from Telegram API

### GitHub Actions Timeout

- This is expected after 6 hours
- Use one of the production deployment options above for 24/7 operation

