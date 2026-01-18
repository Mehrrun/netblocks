# NetBlocks

NetBlocks is a comprehensive network monitoring tool designed to monitor Iranian Autonomous Systems (AS) connectivity via BGP and DNS server availability. It provides both a Telegram bot interface and a command-line interface for real-time network monitoring.

## Features

- **BGP Monitoring**: Real-time monitoring of Iranian AS connectivity using RIPE RIS Live WebSocket API
- **DNS Monitoring**: Continuous monitoring of Iranian DNS servers' availability and response times
- **Traffic Monitoring**: Visual traffic analysis using Cloudflare Radar API with PNG chart generation
- **Telegram Bot**: Interactive bot for checking network status and configuring monitoring intervals
- **CLI Interface**: Command-line tool for monitoring and status reporting
- **Configurable Intervals**: Set custom monitoring intervals via Telegram bot or CLI
- **Periodic Analysis**: Automatic analysis runs every 10 minutes to check network connectivity
- **Readable Output**: Elegant formatting with emojis and clear status indicators
- **Visual Charts**: Professional PNG charts showing Iran's internet traffic trends (24-hour)

## Architecture

The project follows a clean architecture pattern with the following structure:

```
NetBlocks/
├── cmd/
│   ├── cli/           # CLI binary
│   └── telegram-bot/  # Telegram bot binary
├── internal/
│   ├── config/        # Configuration management
│   ├── monitor/       # BGP, DNS, and traffic monitoring logic
│   ├── models/        # Data models
│   └── telegram/      # Telegram bot implementation
├── go.mod
├── Makefile
└── README.md
```

## Prerequisites

- Go 1.21 or higher
- Telegram Bot Token (for Telegram bot functionality)
- Network access to RIPE RIS Live API and DNS servers

## Installation

1. Clone the repository:
```bash
git clone https://github.com/mehrrun/netblocks.git
cd netblocks
```

2. Install dependencies:
```bash
go mod download
```

3. Build the binaries:
```bash
make build
```

Or build individually:
```bash
# Build CLI
go build -o bin/netblocks-cli ./cmd/cli

# Build Telegram Bot
go build -o bin/netblocks-telegram-bot ./cmd/telegram-bot
```

## Configuration

### Configuration File

Create a `config.json` file in the project root (optional - defaults will be used if not provided):

```json
{
  "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "telegram_channel": "@YourChannelUsername",
  "interval": "5m",
  "ris_live_url": "wss://ris-live.ripe.net/v1/ws/?client=netblocks",
  "cloudflare_email": "your-email@example.com",
  "cloudflare_key": "your-cloudflare-api-key",
  "dns_servers": [],
  "iran_asns": []
}
```

### Environment Variables

- `TELEGRAM_BOT_TOKEN`: Telegram bot token (alternative to config file)
- `TELEGRAM_CHANNEL`: Telegram channel username for updates
- `CLOUDFLARE_EMAIL`: Cloudflare account email (for Radar API)
- `CLOUDFLARE_KEY`: Cloudflare API key (for Radar API)

## Usage

### CLI Mode

Run the CLI tool to monitor network status:

```bash
# Run with default interval (5 minutes)
./bin/netblocks-cli

# Run with custom interval
./bin/netblocks-cli -interval 10m

# Run once and exit
./bin/netblocks-cli -once

# Use custom config file
./bin/netblocks-cli -config /path/to/config.json
```

### Telegram Bot Mode

1. Get a Telegram Bot Token from [@BotFather](https://t.me/botfather)

2. Set the token:
```bash
export TELEGRAM_BOT_TOKEN=your_token_here
```

Or add it to `config.json`

3. Run the bot:
```bash
./bin/netblocks-telegram-bot
```

4. Start chatting with your bot on Telegram:
   - `/start` - Welcome message
   - `/status` - Get current monitoring status
   - `/interval <minutes>` - Set monitoring interval (e.g., `/interval 10`)
   - `/help` - Show help message

The bot automatically runs analysis every 10 minutes to check network connectivity.

## Monitoring Details

### BGP Monitoring

NetBlocks monitors Iranian Autonomous Systems by:
- Subscribing to RIPE RIS Live WebSocket API
- Filtering BGP UPDATE messages for Iranian ASNs
- Tracking connectivity status based on recent BGP updates
- Considering an AS disconnected if no updates received in 10 minutes
- Displaying ASN numbers with readable organization names

### DNS Monitoring

DNS monitoring includes:
- Periodic DNS queries to configured servers
- Response time measurement
- Availability status tracking
- Error reporting for failed queries
- Monitoring of authoritative nameservers from .ir domains
- Support for both recursive and authoritative DNS servers
- Distinguishes between network errors and DNS-level responses

### Traffic Monitoring

Traffic monitoring provides:
- Real-time Iran internet traffic analysis via Cloudflare Radar API
- 24-hour traffic trend visualization with PNG charts
- Traffic level percentage calculations
- Change detection (vs baseline)
- Status classification: Normal (>70%), Degraded (30-70%), Throttled (10-30%), Shutdown (<10%)
- Visual charts sent as images in Telegram
- 5-minute caching to avoid API rate limits
- Background refresh every 10 minutes
- Requires Cloudflare API credentials (email + API key)

## Monitored Iranian ASNs

The tool monitors **50 ASNs** including **40 Iranian ASNs** and **10 Cross-Border/Suspicious ASNs**:

### Mobile Operators
- **AS197207** - MCCI (Hamrah-e Avval)
- **AS44244** - Irancell (MTN Irancell)
- **AS57218** - Rightel
- **AS62140** - Rightel Data Center

### TCI/ITC Group
- **AS58224** - TCI (Iran Telecommunication Company)
- **AS12880** - ITC (Information Technology Company)
- **AS49666** - TIC (Telecommunication Infrastructure Company)

### Major ISPs
- **AS31549** - Shatel (Aria Shatel)
- **AS43754** - Asiatech
- **AS51433** - Asiatech (Additional)
- **AS50810** - Mobinnet
- **AS56402** - HiWEB
- **AS16322** - Parsan Lin
- **AS58901** - ParsOnline
- **AS39501** - Sabanet/NGS
- **AS25184** - Afranet
- **AS24631** - Fanap Telecom
- **AS52049** - IranianNet
- **AS49100** - Pishgaman
- **AS206065** - Pasargad Arian
- **AS44400** - Parsian
- **AS50530** - Shabdiz Telecom

### Cloud & CDN Providers (Iranian)
- **AS202468** - Arvan Cloud (Abrarvan)
- **AS42337** - Respina Networks
- **AS202319** - Hezardastan Cloud
- **AS59441** - Hostiran
- **AS8868** - IRCDN

### Global CDN & Cloud Providers
- **AS13335** - Cloudflare (Main)
- **AS14789** - Cloudflare (Secondary)
- **AS202623** - Cloudflare (Core)
- **AS132892** - Cloudflare (Additional)

### Hosting & Datacenter Providers
- **AS25124** - Datak
- **AS205647** - Pardis Fanvari
- **AS49981** - Mabna (Satcomco)
- **AS60631** - ParsPack
- **AS61173** - IranServer
- **AS57067** - Iranian Data Center

### Regional & Municipal ISPs
- **AS56461** - Isfahan Municipality

### Academic & Research Networks
- **AS6736** - IPM (Institute for Research in Fundamental Sciences)
- **AS25306** - IsIran

### Cross-Border / Suspicious ASNs

**Note on Cross-Border ASNs**: These ASNs are registered in Iraq or UAE but show routing behavior suggesting physical infrastructure in Iran, or serve as transit points for Iranian traffic. Monitoring them provides insight into:
- Cross-border network operations
- Traffic routing during internet shutdowns
- Infrastructure masking or jurisdiction shopping
- BGP behavior during censorship events

#### Iraq-Registered
- **AS199739** - Earthlink-DMCC-IQ
- **AS50710** - Earthlink Telecommunications
- **AS59692** - IQWeb FZ-LLC
- **AS203214** - Hulum Almustakbal

#### UAE-Registered
- **AS57568** - ARVANCLOUD GLOBAL (Arvan Cloud Global Infrastructure)
- **AS208800** - G42 CLOUD
- **AS41268** - Sesameware FZ-LLC
- **AS60924** - Orixcom DMCC
- **AS198398** - Symphony Solutions FZ-LLC

#### Historical Registration Issues
- **AS41152** - Ertebatat Fara Gostar Shargh PJSC

## Monitored DNS Servers

The tool monitors **120+ Iranian DNS servers** including both **authoritative nameservers** and **recursive DNS servers**:

### What's the Difference?
- **Authoritative Nameservers**: DNS servers that provide official DNS records for specific domains (e.g., ns1.shatel.ir answers queries about shatel.ir)
- **Recursive DNS Servers**: DNS servers that end-users configure in their network settings for general browsing (e.g., 217.218.127.127 is TCI's public DNS that anyone on their network can use)

### NIC.ir Authoritative Nameservers (.ir TLD)
- `193.189.123.2` - NIC.ir DNS (a.nic.ir)
- `193.189.122.83` - NIC.ir DNS (b.nic.ir)
- `45.93.171.206` - NIC.ir DNS (c.nic.ir)
- `194.225.70.83` - NIC.ir DNS (d.nic.ir)
- `193.0.9.85` - NIC.ir DNS (ir.cctld.authdns.ripe.net)

### Mobile Operators Nameservers
- **Irancell**: ns1-4.mtnirancell.ir (92.42.51.209, 92.42.50.209, 92.42.51.109, 92.42.50.210)
- **MCCI**: ns1-4.mci.ir (5.106.4.129, 5.106.4.130, 5.106.5.129, 5.106.5.130)
- **Rightel**: ns1-4.rightel.ir (185.24.139.91, 185.24.139.71, 185.24.136.90, 91.229.214.232)

### Major ISPs Nameservers (Authoritative)
- **TCI**: ns1-3.tci.ir (194.225.62.1-3)
- **ITC**: ns1-3.itc.ir (194.225.62.10-12)
- **Shatel**: ns1-4.shatel.ir (178.131.80.1-4)
- **Asiatech**: ns1-2.asiatech.ir (185.98.113.141, 185.98.113.142)
- **ParsOnline**: ns1-4.parsonline.ir (194.225.62.80-83)
- **HiWEB**: ns1-4.hiweb.ir (185.51.200.1-4)
- **Mobinnet**: ns1-3.mobinnet.ir (178.22.122.100-102)
- **Sabanet**: ns1-2.sabanet.ir (178.131.88.1-2)
- **Afranet**: ns1-3.afranet.ir (194.225.62.20-22)
- **Fanap**: k.ns.arvancdn.ir, y.ns.arvancdn.ir (185.143.232.253, 185.143.235.253)
- **IranianNet**: ns1-2.iraniannet.ir (178.131.90.1-2)
- **Pishgaman**: ns1-2.pishgaman.net (5.202.129.29-30)
- **Pasargad Arian**: ns1-2.pasargad.ir (185.55.229.1-2)
- **Parsian**: ns1-2.parsian.ir (178.131.92.1-2)
- **Shabdiz Telecom**: ns1-2.shabdiz.ir (185.55.230.1-2)

### Cloud & CDN Providers Nameservers
- **Arvan Cloud**: ns1.arvancdn.ir, ns2.arvancdn.ir (185.143.232.253, 185.143.235.253)
- **Respina**: Uses Cloudflare nameservers (172.64.32.171, 172.64.35.251)
- **Hezardastan Cloud**: ns.sotoon53.com, h.ns.sotoon53.com (194.34.163.53, 185.166.104.53)
- **Hostiran**: ns1-2.hostiran.net (37.27.81.177, 5.144.130.130)
- **IRCDN**: Uses Cloudflare nameservers (108.162.194.236, 172.64.35.109)

### Datacenter Providers Nameservers
- **Datak**: ns1-4.datak.ir (81.91.129.230, 81.91.129.229, 81.91.129.226, 81.91.129.227)
- **Pardis Fanvari**: ns1-2.pardis.ir (185.143.235.1-2)
- **Mabna (Satcomco)**: ns1-2.satcomco.com (45.14.135.25)
- **ParsPack**: Uses CloudNS nameservers (109.201.133.251, 185.206.180.55, 178.156.179.118, 51.91.57.244)
- **IranServer**: Uses Cloudflare nameservers (108.162.193.143, 173.245.58.184)
- **Iranian Data Center**: 176.62.144.44

### Academic & Research Nameservers
- **IPM**: ns1-3.ipm.ir (194.225.62.60-62)
- **IsIran**: ns1-2.isiran.ir (194.225.62.70-71)

### Regional & Municipal Nameservers
- **Isfahan Municipality**: ns1-2.isfahan.ir (194.225.62.75-76)

### Public DNS Services (Recursive)
- **Shecan DNS**: 178.22.122.100, 185.51.200.2, 178.22.122.101, 185.51.200.1

---

## Recursive DNS Servers (Public Resolvers)

These are the DNS servers that Iranian end-users actually configure in their network settings for browsing:

### TCI/ITC/TIC Group Recursive DNS
- **TCI (Mokhaberat)**: 
  - Primary: `217.218.127.127`
  - Secondary: `217.218.155.155`
  - Regional: `80.191.40.41`
- **TIC (Infrastructure Co.)**: `2.189.44.44`
- **ITC (Information Technology Co.)**: `2.188.21.130`

### Major ISP Recursive DNS
- **Shatel**: 
  - Primary: `85.15.1.10`
  - Secondary: `85.15.1.12`
- **Asiatech**: 
  - Primary: `194.225.150.10`
  - Secondary: `194.225.150.20`
- **ParsOnline**: `91.99.101.12`

### Anti-Sanction & Gaming DNS Services
- **403.online** (Anti-Sanction):
  - Primary: `10.202.10.202` *(Private IP - accessible only within Iranian networks)*
  - Secondary: `10.202.10.102` *(Private IP - accessible only within Iranian networks)*
- **Electro** (Anti-Sanction/Gaming):
  - Primary: `78.157.42.100`
  - Secondary: `78.157.42.101`
- **Radar Game**: `10.202.10.10` *(Private IP - accessible only within Iranian networks)*
- **Begzar** (Anti-Sanction):
  - Primary: `185.55.226.26`
  - Secondary: `185.55.226.25`

### Cloud Provider Recursive DNS
- **ArvanCloud**: `185.97.117.187`
- **Shahrad / Sefroyek**: `185.51.200.50`

### Academic & Research Recursive DNS
- **IRIPM** (Institute for Research): `194.225.73.141` (persia.iranet.ir)
- **IROST** (Research Organization): `213.176.123.5`
- **TUMS** (Tehran University of Medical Sciences): `194.225.62.80` (ourdns1.tums.ac.ir)

### Regional & Municipal Recursive DNS
- **Tehran Municipality ICT**:
  - Primary: `31.24.234.34`
  - Secondary: `31.24.234.35`
  - Tertiary: `31.24.234.37`
- **Kish Cell Pars (KCP Cloud)**: `91.245.229.1`

### Other Providers
- **Hamkaran System**: `185.187.84.15`
- **Tehran Public DNS**: `37.156.145.229`
- **Datak**: ns1-4.datak.ir (81.91.129.230, 81.91.129.229, 81.91.129.226, 81.91.129.227)
- **Pardis Fanvari**: ns1-2.pardis.ir (185.143.235.1-2)
- **Mabna**: ns1-2.satcomco.com (45.14.135.25)
- **ParsPack**: Uses CloudNS nameservers (109.201.133.251, 185.206.180.55, 178.156.179.118, 51.91.57.244)
- **IranServer**: Uses Cloudflare nameservers (108.162.193.143, 173.245.58.184)
- **Iranian Data Center**: irandatacenter.ir (176.62.144.44)

### Academic & Research Networks Nameservers
- **IPM**: ns1-3.ipm.ir (194.225.62.60-62)
- **IsIran**: ns1-2.isiran.ir (194.225.62.70-71)

### Public DNS Services
- **Shecan DNS**: 178.22.122.100, 185.51.200.2, 178.22.122.101, 185.51.200.1

## Development

### Project Structure

- `cmd/cli/`: CLI application entry point
- `cmd/telegram-bot/`: Telegram bot entry point
- `internal/config/`: Configuration loading and management
- `internal/monitor/`: Core monitoring logic
  - `bgp.go`: RIS Live BGP monitoring client
  - `dns.go`: DNS server monitoring
  - `monitor.go`: Coordinator for all monitoring
- `internal/models/`: Data structures
- `internal/telegram/`: Telegram bot implementation

### Building

```bash
# Build all binaries
make build

# Build CLI only
make build-cli

# Build Telegram bot only
make build-bot

# Clean build artifacts
make clean
```

### Running Tests

```bash
go test ./...
```

## API Reference

### RIS Live API

NetBlocks uses the [RIPE RIS Live WebSocket API](https://ris-live.ripe.net/manual/) for BGP monitoring. The implementation:
- Connects to `wss://ris-live.ripe.net/v1/ws/`
- Subscribes to BGP UPDATE messages for specific ASNs
- Processes real-time BGP routing information

### DNS Protocol

DNS monitoring uses standard DNS queries (A record lookups for `leader.ir`) to test server availability. The tool queries authoritative nameservers directly to check their responsiveness.

### Cloudflare Radar API

Traffic monitoring uses the [Cloudflare Radar API](https://developers.cloudflare.com/radar/) for Iran's internet traffic data:
- Endpoint: `https://api.cloudflare.com/client/v4/radar/http/timeseries_groups/bandwidth`
- Requires authentication: Cloudflare email + API key
- Get your API key from: https://dash.cloudflare.com/profile/api-tokens
- 24-hour historical data with 1-hour aggregation intervals
- Chart generation using [go-chart library](https://github.com/wcharczuk/go-chart)
- Set credentials in `config.json` or environment variables

## Output Format

### CLI Output
- 🟢 Green circle = Connected/Alive
- 🔴 Red circle = Disconnected/Down
- Sorted output (connected/alive entries first)
- Summary statistics with emojis

### Telegram Bot Output
- Markdown formatted messages
- 🟢 Green circle = Connected/Alive
- 🔴 Red circle = Disconnected/Down
- Hierarchical display with tree-style formatting
- Summary statistics
- **Traffic charts sent as PNG images** with:
  - 800x400px line chart
  - 24-hour traffic trend
  - Color-coded status (Green=Normal, Yellow=Degraded, Orange=Throttled, Red=Shutdown)
  - Traffic level percentage
  - Change percentage vs baseline

## Troubleshooting

### Connection Issues

- Ensure you have internet connectivity
- Check firewall settings for WebSocket connections
- Verify DNS servers are reachable

### Telegram Bot Not Responding

- Verify bot token is correct
- Check bot is running and connected
- Ensure bot has necessary permissions

### No BGP Updates

- Wait a few minutes for initial BGP data
- Check if monitored ASNs are active
- Verify RIS Live API connectivity

### DNS Servers Showing as Down

- Some nameservers may not be publicly resolvable from outside Iran
- DNS queries use `leader.ir` as the test domain
- Check if you're running from within Iran for accurate results

### Traffic Chart Not Showing

- Ensure Cloudflare credentials are configured in `config.json`
- Get API key from https://dash.cloudflare.com/profile/api-tokens
- Check bot logs for API authentication errors
- Chart generation requires go-chart library dependencies
- Ensure bot has permission to send photos in the channel

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [RIPE NCC](https://www.ripe.net/) for providing RIS Live API
- [RIPE RIS Live Documentation](https://ris-live.ripe.net/manual/)
- Go Telegram Bot API community

## Support

For issues, questions, or contributions, please open an issue on [GitHub](https://github.com/mehrrun/netblocks).

## Author

**mehrrun** - [GitHub](https://github.com/mehrrun)

## Disclaimer

This tool is for monitoring and educational purposes. Network connectivity status is based on BGP routing information and DNS queries, which may not always reflect actual end-user connectivity. Some DNS servers may not be publicly resolvable from outside Iran.
