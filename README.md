# NetBlocks

NetBlocks is a comprehensive network monitoring tool designed to monitor Iranian Autonomous Systems (AS) connectivity via BGP and DNS server availability. It provides both a Telegram bot interface and a command-line interface for real-time network monitoring.

## Features

- **BGP Monitoring**: Real-time monitoring of Iranian AS connectivity using RIPE RIS Live WebSocket API
- **DNS Monitoring**: Continuous monitoring of Iranian DNS servers' availability and response times
- **Telegram Bot**: Interactive bot for checking network status and configuring monitoring intervals
- **CLI Interface**: Command-line tool for monitoring and status reporting
- **Configurable Intervals**: Set custom monitoring intervals via Telegram bot or CLI
- **Periodic Analysis**: Automatic analysis runs every 10 minutes to check network connectivity
- **Readable Output**: Elegant formatting with emojis and clear status indicators

## Architecture

The project follows a clean architecture pattern with the following structure:

```
NetBlocks/
├── cmd/
│   ├── cli/           # CLI binary
│   └── telegram-bot/  # Telegram bot binary
├── internal/
│   ├── config/        # Configuration management
│   ├── monitor/       # BGP and DNS monitoring logic
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
  "interval": "5m",
  "ris_live_url": "wss://ris-live.ripe.net/v1/ws/?client=netblocks",
  "dns_servers": [],
  "iran_asns": []
}
```

### Environment Variables

- `TELEGRAM_BOT_TOKEN`: Telegram bot token (alternative to config file)

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

## Monitored Iranian ASNs

The tool monitors **40 Iranian ASNs** including:

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

## Monitored DNS Servers

The tool monitors **80+ Iranian DNS servers** including:

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

### Major ISPs Nameservers
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
