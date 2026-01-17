# Railway.app Deployment Guide

## Quick Setup (5 minutes)

### Step 1: Sign up for Railway
1. Go to https://railway.app
2. Click "Start a New Project"
3. Sign up with GitHub (recommended) or email

### Step 2: Create New Project
1. Click "New Project"
2. Select "Deploy from GitHub repo"
3. Find and select your `netblocks` repository
4. Click "Deploy Now"

### Step 3: Configure Environment Variables
Railway will start building. While it builds:

1. Go to your project → **Variables** tab
2. Add these environment variables:

   **Variable 1:**
   - Name: `TELEGRAM_BOT_TOKEN`
   - Value: `8559677536:AAEyV4CpmhurbTqaKWJkcW_nb2rQsKerjDo`

   **Variable 2:**
   - Name: `TELEGRAM_CHANNEL`
   - Value: `IranBlackoutMonitor`

3. Click "Add" for each variable

### Step 4: Configure Service Settings
1. Go to **Settings** tab
2. Under **Deploy**, set:
   - **Start Command**: `./netblocks-telegram-bot`
   - **Root Directory**: `/` (default)

### Step 5: Deploy
1. Railway will automatically detect the Go project
2. It will build using the `nixpacks.toml` or `railway.json` config
3. The bot will start automatically after build completes

### Step 6: Verify Deployment
1. Check the **Deployments** tab
2. Click on the latest deployment
3. Check **Logs** tab - you should see:
   ```
   Authorized on account @YourBotName
   Channel updates enabled for: @IranBlackoutMonitor
   Periodic updates started...
   ```

### Step 7: Test the Bot
1. Open Telegram
2. Find your bot
3. Send `/start` or `/status`
4. Check your channel `@IranBlackoutMonitor` for periodic updates

## Monitoring

- **Logs**: View real-time logs in Railway dashboard
- **Metrics**: Check CPU, Memory, Network usage
- **Deployments**: See deployment history and status

## Troubleshooting

### Bot Not Starting
- Check logs for errors
- Verify environment variables are set correctly
- Ensure bot token is valid

### Bot Stops Unexpectedly
- Check Railway logs for errors
- Verify network connectivity
- Check if bot is hitting rate limits

### Updates Not Working
- Verify `TELEGRAM_CHANNEL` is set correctly
- Check bot has admin permissions in channel
- Review logs for sending errors

## Cost

Railway offers:
- **Free tier**: $5 credit/month
- **Pay-as-you-go**: After free tier
- **Estimated cost**: ~$2-5/month for this bot (very low resource usage)

## Auto-Deploy from GitHub

Railway automatically deploys when you push to `main` branch!

1. Make changes to your code
2. Push to GitHub
3. Railway detects the push
4. Automatically rebuilds and redeploys
5. Zero downtime deployment

## Custom Domain (Optional)

Railway provides a free `.railway.app` domain. You can also add your own custom domain in the **Settings** tab (not needed for Telegram bot, but available).

