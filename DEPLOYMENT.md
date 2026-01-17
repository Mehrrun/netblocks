# Deployment Guide

## Current Deployment: GitHub Actions with systemd Daemon

The bot runs as a **systemd service** on GitHub Actions runners, providing proper daemon functionality with automatic restarts and service management.

### How It Works

1. The workflow builds the bot binary
2. Creates a systemd service file with environment variables
3. Starts the bot as a systemd daemon
4. Monitors the service status every 5 minutes
5. Automatically restarts the bot if it crashes
6. Logs to systemd journal (accessible via `journalctl`)

### Advantages

- ✅ **Proper Daemon**: Bot runs as a true system service
- ✅ **Auto-Restart**: Automatically restarts on crashes (RestartSec=10)
- ✅ **Log Management**: Structured logs via journalctl
- ✅ **Service Control**: Standard systemctl commands
- ✅ **Status Monitoring**: Periodic health checks every 5 minutes
- ✅ **Environment Variables**: Securely passed from GitHub secrets

### Limitations

- ⏱️ **6-hour timeout limit** on GitHub Actions free tier
- 💰 **Usage limits** on free tier
- 🔄 **Workflow cancellation** stops the daemon when repository is inactive

### Monitoring

Check bot status on GitHub Actions:

1. Go to Actions tab in GitHub repository
2. View the latest "Build and Deploy NetBlocks Bot" workflow
3. Check the "Monitor bot daemon" step for logs
4. Status checks run every 5 minutes showing bot health

View logs from workflow:
```bash
# The workflow automatically shows:
# - Initial 50 lines of logs
# - Status check every 5 minutes
# - Recent 20 lines from last 5 minutes
```

### Local Testing with systemd

You can test the systemd service locally before deploying:

```bash
# 1. Build the bot
go build -o netblocks-telegram-bot ./cmd/telegram-bot

# 2. Edit the service file with your paths
nano netblocks-bot.service
# Update WorkingDirectory and ExecStart paths
# Add your TELEGRAM_BOT_TOKEN and TELEGRAM_CHANNEL

# 3. Install service
sudo cp netblocks-bot.service /etc/systemd/system/
sudo systemctl daemon-reload

# 4. Start service
sudo systemctl start netblocks-bot

# 5. Check status
sudo systemctl status netblocks-bot

# 6. View logs
sudo journalctl -u netblocks-bot -f

# 7. Stop service
sudo systemctl stop netblocks-bot

# 8. Restart service
sudo systemctl restart netblocks-bot
```

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

- **GitHub Actions (systemd)**: 
  - View workflow logs in Actions tab
  - Status checks every 5 minutes
  - `sudo journalctl -u netblocks-bot` (in workflow)
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

### systemd Service Issues (GitHub Actions)

1. **Service fails to start**:
   ```bash
   # Check service status in workflow logs
   sudo systemctl status netblocks-bot --no-pager --full
   sudo journalctl -u netblocks-bot -n 100 --no-pager
   ```

2. **Bot crashes and doesn't restart**:
   - Check RestartSec setting in service file
   - Verify Restart=always is set
   - Check for fatal errors in logs

3. **Environment variables not working**:
   - Verify secrets are set in GitHub repository
   - Check service file shows correct environment variables
   - View with: `cat /etc/systemd/system/netblocks-bot.service`

### GitHub Actions Timeout

- This is expected after 6 hours on free tier
- Use one of the production deployment options above for 24/7 operation
- Or upgrade to GitHub Actions Pro for longer runs

