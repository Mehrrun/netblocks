package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	TelegramToken string        `json:"telegram_token"`
	TelegramChannel string      `json:"telegram_channel,omitempty"` // Channel username (e.g., @IranBlackoutMonitor) or chat ID
	Interval      time.Duration `json:"interval"`
	RISLiveURL    string        `json:"ris_live_url"`
	DNSServers    []DNSServer   `json:"dns_servers"`
	IranASNs      []string      `json:"iran_asns"`
}

// UnmarshalJSON implements custom JSON unmarshaling for Config
func (c *Config) UnmarshalJSON(data []byte) error {
	// Use a temporary struct to handle the interval as string
	type Alias Config
	aux := &struct {
		Interval string `json:"interval"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse interval string to time.Duration
	if aux.Interval != "" {
		duration, err := time.ParseDuration(aux.Interval)
		if err != nil {
			return err
		}
		c.Interval = duration
	} else {
		c.Interval = 5 * time.Minute // Default
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling for Config
func (c Config) MarshalJSON() ([]byte, error) {
	type Alias Config
	return json.Marshal(&struct {
		Interval string `json:"interval"`
		*Alias
	}{
		Interval: c.Interval.String(),
		Alias:    (*Alias)(&c),
	})
}

// DNSServer represents a DNS server configuration
type DNSServer struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"` // "recursive", "authoritative", or "both" (default: "both")
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Interval:   5 * time.Minute,
		RISLiveURL: "wss://ris-live.ripe.net/v1/ws/?client=netblocks",
		DNSServers: GetDefaultIranianDNSServers(),
		IranASNs:   GetDefaultIranianASNs(),
	}
}

// LoadConfig loads configuration from a JSON file, or returns default if file doesn't exist
func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults if empty
	if config.RISLiveURL == "" {
		config.RISLiveURL = "wss://ris-live.ripe.net/v1/ws/?client=netblocks"
	}
	if len(config.DNSServers) == 0 {
		config.DNSServers = GetDefaultIranianDNSServers()
	}
	if len(config.IranASNs) == 0 {
		config.IranASNs = GetDefaultIranianASNs()
	}

	return &config, nil
}

// SaveConfig saves configuration to a JSON file
func SaveConfig(path string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetDefaultIranianDNSServers returns a comprehensive list of Iranian DNS servers
// Includes authoritative nameservers and recursive DNS servers from ISPs, datacenters, and cloud providers
func GetDefaultIranianDNSServers() []DNSServer {
	return []DNSServer{
		// ============================================
		// NIC.ir AUTHORITATIVE NAMESERVERS (.ir TLD)
		// ============================================
		{Address: "193.189.123.2", Name: "NIC.ir DNS (a.nic.ir)", Type: "authoritative"},
		{Address: "193.189.122.83", Name: "NIC.ir DNS (b.nic.ir)", Type: "authoritative"},
		{Address: "45.93.171.206", Name: "NIC.ir DNS (c.nic.ir)", Type: "authoritative"},
		{Address: "194.225.70.83", Name: "NIC.ir DNS (d.nic.ir)", Type: "authoritative"},
		{Address: "193.0.9.85", Name: "NIC.ir DNS (ir.cctld.authdns.ripe.net)", Type: "authoritative"},

		// ============================================
		// MOBILE OPERATORS - DNS SERVERS (Nameservers)
		// ============================================
		// Irancell (MTN Irancell) - irancell.ir
		{Address: "92.42.51.209", Name: "Irancell DNS (ns1.mtnirancell.ir)", Type: "authoritative"},
		{Address: "92.42.50.209", Name: "Irancell DNS (ns2.mtnirancell.ir)", Type: "authoritative"},
		{Address: "92.42.51.109", Name: "Irancell DNS (ns3.mtnirancell.ir)", Type: "authoritative"},
		{Address: "92.42.50.210", Name: "Irancell DNS (ns4.mtnirancell.ir)", Type: "authoritative"},

		// MCCI (Hamrah-e Avval / Mobile Communication Company of Iran) - mci.ir
		{Address: "5.106.4.129", Name: "MCCI DNS (ns1.mci.ir)", Type: "authoritative"},
		{Address: "5.106.4.130", Name: "MCCI DNS (ns2.mci.ir)", Type: "authoritative"},
		{Address: "5.106.5.129", Name: "MCCI DNS (ns3.mci.ir)", Type: "authoritative"},
		{Address: "5.106.5.130", Name: "MCCI DNS (ns4.mci.ir)", Type: "authoritative"},

		// Rightel Communication Service Company - rightel.ir
		{Address: "185.24.139.91", Name: "Rightel DNS (ns1.rightel.ir)", Type: "authoritative"},
		{Address: "185.24.139.71", Name: "Rightel DNS (ns2.rightel.ir)", Type: "authoritative"},
		{Address: "185.24.136.90", Name: "Rightel DNS (ns3.rightel.ir)", Type: "authoritative"},
		{Address: "91.229.214.232", Name: "Rightel DNS (ns4.rightel.ir)", Type: "authoritative"},

		// ============================================
		// TCI/ITC GROUP - DNS SERVERS
		// ============================================
		// Iran Telecommunication Company (TCI) - tci.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.1", Name: "TCI DNS (ns1.tci.ir)", Type: "authoritative"},
		{Address: "194.225.62.2", Name: "TCI DNS (ns2.tci.ir)", Type: "authoritative"},
		{Address: "194.225.62.3", Name: "TCI DNS (ns3.tci.ir)", Type: "authoritative"},

		// Information Technology Company (ITC) - itc.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.10", Name: "ITC DNS (ns1.itc.ir)", Type: "authoritative"},
		{Address: "194.225.62.11", Name: "ITC DNS (ns2.itc.ir)", Type: "authoritative"},
		{Address: "194.225.62.12", Name: "ITC DNS (ns3.itc.ir)", Type: "authoritative"},

		// ============================================
		// SHATEL GROUP - DNS SERVERS (Nameservers)
		// ============================================
		// Aria Shatel PJSC - shatel.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "178.131.80.1", Name: "Shatel DNS (ns1.shatel.ir)", Type: "authoritative"},
		{Address: "178.131.80.2", Name: "Shatel DNS (ns2.shatel.ir)", Type: "authoritative"},
		{Address: "178.131.80.3", Name: "Shatel DNS (ns3.shatel.ir)", Type: "authoritative"},
		{Address: "178.131.80.4", Name: "Shatel DNS (ns4.shatel.ir)", Type: "authoritative"},

		// ============================================
		// ASIATECH GROUP - DNS SERVERS (Nameservers)
		// ============================================
		// Asiatech Data Transmission Company - asiatech.ir
		{Address: "185.98.113.141", Name: "Asiatech DNS (ns1.asiatech.ir)", Type: "authoritative"},
		{Address: "185.98.113.142", Name: "Asiatech DNS (ns2.asiatech.ir)", Type: "authoritative"},

		// ============================================
		// MAJOR ISPs - DNS SERVERS (Nameservers)
		// ============================================
		// Parsan Lin Co. PJS / ParsOnline - parsonline.com, parsonline.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.80", Name: "ParsOnline DNS (ns1.parsonline.ir)", Type: "authoritative"},
		{Address: "194.225.62.81", Name: "ParsOnline DNS (ns2.parsonline.ir)", Type: "authoritative"},
		{Address: "194.225.62.82", Name: "ParsOnline DNS (ns3.parsonline.ir)", Type: "authoritative"},
		{Address: "194.225.62.83", Name: "ParsOnline DNS (ns4.parsonline.ir)", Type: "authoritative"},

		// Dadeh Gostar Asr Novin Co (HiWEB) - hiweb.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "185.51.200.1", Name: "HiWEB DNS (ns1.hiweb.ir)", Type: "authoritative"},
		{Address: "185.51.200.2", Name: "HiWEB DNS (ns2.hiweb.ir)", Type: "authoritative"},
		{Address: "185.51.200.3", Name: "HiWEB DNS (ns3.hiweb.ir)", Type: "authoritative"},
		{Address: "185.51.200.4", Name: "HiWEB DNS (ns4.hiweb.ir)", Type: "authoritative"},

		// Mobinnet Communication Company - mobinnet.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "178.22.122.100", Name: "Mobinnet DNS (ns1.mobinnet.ir)", Type: "authoritative"},
		{Address: "178.22.122.101", Name: "Mobinnet DNS (ns2.mobinnet.ir)", Type: "authoritative"},
		{Address: "178.22.122.102", Name: "Mobinnet DNS (ns3.mobinnet.ir)", Type: "authoritative"},

		// Parvaresh Dadeha Co. (Sabanet/NGS-NedaGostarSaba) - sabanet.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "178.131.88.1", Name: "Sabanet DNS (ns1.sabanet.ir)", Type: "authoritative"},
		{Address: "178.131.88.2", Name: "Sabanet DNS (ns2.sabanet.ir)", Type: "authoritative"},

		// Afranet - afranet.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.20", Name: "Afranet DNS (ns1.afranet.ir)", Type: "authoritative"},
		{Address: "194.225.62.21", Name: "Afranet DNS (ns2.afranet.ir)", Type: "authoritative"},
		{Address: "194.225.62.22", Name: "Afranet DNS (ns3.afranet.ir)", Type: "authoritative"},

		// Fanap Telecom - fanap.ir
		{Address: "185.143.232.253", Name: "Fanap DNS (k.ns.arvancdn.ir)", Type: "authoritative"},
		{Address: "185.143.235.253", Name: "Fanap DNS (y.ns.arvancdn.ir)", Type: "authoritative"},

		// IranianNet Communication and Electronic Services Co - iraniannet.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "178.131.90.1", Name: "IranianNet DNS (ns1.iraniannet.ir)", Type: "authoritative"},
		{Address: "178.131.90.2", Name: "IranianNet DNS (ns2.iraniannet.ir)", Type: "authoritative"},

		// Pishgaman Toseeh Ertebatat Co - pishgaman.ir
		{Address: "5.202.129.29", Name: "Pishgaman DNS (ns1.pishgaman.net)", Type: "authoritative"},
		{Address: "5.202.129.30", Name: "Pishgaman DNS (ns2.pishgaman.net)", Type: "authoritative"},

		// Tose'h Fanavari Ertebatat Pasargad Arian Co - pasargad.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "185.55.229.1", Name: "Pasargad Arian DNS (ns1.pasargad.ir)", Type: "authoritative"},
		{Address: "185.55.229.2", Name: "Pasargad Arian DNS (ns2.pasargad.ir)", Type: "authoritative"},

		// Ertebatat Sabet Parsian Co - parsian.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "178.131.92.1", Name: "Parsian DNS (ns1.parsian.ir)", Type: "authoritative"},
		{Address: "178.131.92.2", Name: "Parsian DNS (ns2.parsian.ir)", Type: "authoritative"},

		// Shabdiz Telecom Network PJSC - shabdiz.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "185.55.230.1", Name: "Shabdiz Telecom DNS (ns1.shabdiz.ir)", Type: "authoritative"},
		{Address: "185.55.230.2", Name: "Shabdiz Telecom DNS (ns2.shabdiz.ir)", Type: "authoritative"},

		// ============================================
		// ADDITIONAL DATACENTERS & HOSTING - DNS SERVERS
		// ============================================
		// Mabna (Satcomco) - mabna.ir
		{Address: "45.14.135.25", Name: "Mabna DNS (ns1.satcomco.com)", Type: "authoritative"},
		{Address: "45.14.135.25", Name: "Mabna DNS (ns2.satcomco.com)", Type: "authoritative"},

		// ParsPack (Vandad Vira Hooman LLC) - parspack.ir
		// Note: Uses CloudNS nameservers
		{Address: "109.201.133.251", Name: "ParsPack DNS (ns71.cloudns.net)", Type: "authoritative"},
		{Address: "185.206.180.55", Name: "ParsPack DNS (ns74.cloudns.uk)", Type: "authoritative"},
		{Address: "178.156.179.118", Name: "ParsPack DNS (ns72.cloudns.com)", Type: "authoritative"},
		{Address: "51.91.57.244", Name: "ParsPack DNS (ns73.cloudns.net)", Type: "authoritative"},

		// IranServer (Green Web Samaneh Novin PJSC) - iranserver.com
		// Note: Uses Cloudflare nameservers
		{Address: "108.162.193.143", Name: "IranServer DNS (sid.ns.cloudflare.com)", Type: "authoritative"},
		{Address: "173.245.58.184", Name: "IranServer DNS (leia.ns.cloudflare.com)", Type: "authoritative"},

		// Iranian Data Center (KEYANA Information Technology Co. Ltd.) - irandatacenter.ir
		// Note: DNS servers need to be discovered via domain resolution
		{Address: "176.62.144.44", Name: "Iranian Data Center DNS (irandatacenter.ir)", Type: "authoritative"},

		// ============================================
		// CLOUD & CDN PROVIDERS - DNS SERVERS (Nameservers)
		// ============================================
		// Arvan Cloud / Abrarvan (Noyan Abr Arvan Co) - arvancdn.ir, arvancloud.ir
		{Address: "185.143.232.253", Name: "Arvan Cloud DNS (ns1.arvancdn.ir)", Type: "authoritative"},
		{Address: "185.143.235.253", Name: "Arvan Cloud DNS (ns2.arvancdn.ir)", Type: "authoritative"},
		{Address: "185.143.232.253", Name: "Arvan Cloud DNS (k.ns.arvancdn.ir)", Type: "authoritative"},
		{Address: "185.143.235.253", Name: "Arvan Cloud DNS (y.ns.arvancdn.ir)", Type: "authoritative"},

		// Respina Networks & Beyond PJSC - respina.ir
		// Note: Uses Cloudflare nameservers (jessica.ns.cloudflare.com, marvin.ns.cloudflare.com)
		{Address: "172.64.32.171", Name: "Respina DNS (jessica.ns.cloudflare.com)", Type: "authoritative"},
		{Address: "172.64.35.251", Name: "Respina DNS (marvin.ns.cloudflare.com)", Type: "authoritative"},

		// Hezardastan Unit Cloud Computing PJSC - hezardastan.ir
		{Address: "194.34.163.53", Name: "Hezardastan Cloud DNS (ns.sotoon53.com)", Type: "authoritative"},
		{Address: "185.166.104.53", Name: "Hezardastan Cloud DNS (h.ns.sotoon53.com)", Type: "authoritative"},

		// Hostiran-Network (Noavaran Shabakeh Sabz Mehregan) - hostiran.ir
		{Address: "37.27.81.177", Name: "Hostiran DNS (ns1.hostiran.net)", Type: "authoritative"},
		{Address: "5.144.130.130", Name: "Hostiran DNS (ns2.hostiran.net)", Type: "authoritative"},

		// Khallagh Borhan Market Development (IRCDN) - ircdn.ir
		// Note: Uses Cloudflare nameservers (heidi.ns.cloudflare.com, randy.ns.cloudflare.com)
		{Address: "108.162.194.236", Name: "IRCDN DNS (heidi.ns.cloudflare.com)", Type: "authoritative"},
		{Address: "172.64.35.109", Name: "IRCDN DNS (randy.ns.cloudflare.com)", Type: "authoritative"},

		// ============================================
		// HOSTING & DATACENTER PROVIDERS - DNS SERVERS (Nameservers)
		// ============================================
		// Datak Company LLC - datak.ir
		{Address: "81.91.129.230", Name: "Datak DNS (ns1.datak.ir)", Type: "authoritative"},
		{Address: "81.91.129.229", Name: "Datak DNS (ns2.datak.ir)", Type: "authoritative"},
		{Address: "81.91.129.226", Name: "Datak DNS (ns3.datak.ir)", Type: "authoritative"},
		{Address: "81.91.129.227", Name: "Datak DNS (ns4.datak.ir)", Type: "authoritative"},

		// Pardis Fanvari Partak Ltd - pardis.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "185.143.235.1", Name: "Pardis Fanvari DNS (ns1.pardis.ir)", Type: "authoritative"},
		{Address: "185.143.235.2", Name: "Pardis Fanvari DNS (ns2.pardis.ir)", Type: "authoritative"},

		// ============================================
		// ACADEMIC & RESEARCH NETWORKS - DNS SERVERS
		// ============================================
		// Institute for Research in Fundamental Sciences (IPM/IPMI) - ipm.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.60", Name: "IPM DNS (ns1.ipm.ir)", Type: "authoritative"},
		{Address: "194.225.62.61", Name: "IPM DNS (ns2.ipm.ir)", Type: "authoritative"},
		{Address: "194.225.62.62", Name: "IPM DNS (ns3.ipm.ir)", Type: "authoritative"},

		// IsIran (Education/ISP) - isiran.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.70", Name: "IsIran DNS (ns1.isiran.ir)", Type: "authoritative"},
		{Address: "194.225.62.71", Name: "IsIran DNS (ns2.isiran.ir)", Type: "authoritative"},

		// ============================================
		// REGIONAL & MUNICIPAL ISPs - DNS SERVERS
		// ============================================
		// Information Technology Organization of Isfahan Municipality - isfahan.ir
		// Note: Nameservers may not be publicly resolvable from outside Iran
		{Address: "194.225.62.75", Name: "Isfahan Municipality DNS (ns1.isfahan.ir)", Type: "authoritative"},
		{Address: "194.225.62.76", Name: "Isfahan Municipality DNS (ns2.isfahan.ir)", Type: "authoritative"},

		// ============================================
		// PUBLIC DNS SERVICES (Iranian)
		// ============================================
		// Shecan DNS (Public DNS service)
		{Address: "178.22.122.100", Name: "Shecan DNS (Primary)", Type: "recursive"},
		{Address: "185.51.200.2", Name: "Shecan DNS (Secondary)", Type: "recursive"},
		{Address: "178.22.122.101", Name: "Shecan DNS (Tertiary)", Type: "recursive"},
		{Address: "185.51.200.1", Name: "Shecan DNS (Quaternary)", Type: "recursive"},

		// ============================================
		// RECURSIVE DNS SERVERS (Public Resolvers)
		// These are DNS servers that end-users within Iranian networks use for browsing
		// ============================================

		// ============================================
		// TCI/ITC/TIC GROUP - RECURSIVE DNS
		// ============================================
		// Iran Telecommunication Company (TCI / Mokhaberat)
		{Address: "217.218.127.127", Name: "TCI Recursive DNS (Primary)", Type: "recursive"},
		{Address: "217.218.155.155", Name: "TCI Recursive DNS (Secondary)", Type: "recursive"},
		{Address: "80.191.40.41", Name: "TCI Recursive DNS (Regional)", Type: "recursive"},

		// Telecommunication Infrastructure Company (TIC)
		{Address: "2.189.44.44", Name: "TIC Recursive DNS", Type: "recursive"},

		// Information Technology Company (ITC)
		{Address: "2.188.21.130", Name: "ITC Recursive DNS", Type: "recursive"},

		// ============================================
		// MAJOR ISP RECURSIVE DNS
		// ============================================
		// Aria Shatel PJSC
		{Address: "85.15.1.10", Name: "Shatel Recursive DNS (Primary)", Type: "recursive"},
		{Address: "85.15.1.12", Name: "Shatel Recursive DNS (Secondary)", Type: "recursive"},

		// Asiatech Data Transmission Company
		{Address: "194.225.150.10", Name: "Asiatech Recursive DNS (Primary)", Type: "recursive"},
		{Address: "194.225.150.20", Name: "Asiatech Recursive DNS (Secondary)", Type: "recursive"},

		// Parsan Lin Co. PJS / ParsOnline
		{Address: "91.99.101.12", Name: "ParsOnline Recursive DNS", Type: "recursive"},

		// ============================================
		// ANTI-SANCTION & GAMING DNS SERVICES
		// ============================================
		// 403.online (Anti-Sanction DNS Service)
		// Note: Private IPs (10.x.x.x) only accessible from within Iranian networks
		{Address: "10.202.10.202", Name: "403.online DNS (Primary)", Type: "recursive"},
		{Address: "10.202.10.102", Name: "403.online DNS (Secondary)", Type: "recursive"},

		// Electro (Anti-Sanction/Gaming DNS Service)
		{Address: "78.157.42.100", Name: "Electro DNS (Primary)", Type: "recursive"},
		{Address: "78.157.42.101", Name: "Electro DNS (Secondary)", Type: "recursive"},

		// Radar Game (Gaming DNS Service)
		// Note: Private IP (10.x.x.x) only accessible from within Iranian networks
		{Address: "10.202.10.10", Name: "Radar Game DNS", Type: "recursive"},

		// Begzar (Anti-Sanction DNS Service)
		{Address: "185.55.226.26", Name: "Begzar DNS (Primary)", Type: "recursive"},
		{Address: "185.55.226.25", Name: "Begzar DNS (Secondary)", Type: "recursive"},

		// ============================================
		// CLOUD PROVIDER RECURSIVE DNS
		// ============================================
		// Arvan Cloud / Abrarvan (Noyan Abr Arvan Co)
		{Address: "185.97.117.187", Name: "ArvanCloud Recursive DNS", Type: "recursive"},

		// Shahrad / Sefroyek
		{Address: "185.51.200.50", Name: "Shahrad/Sefroyek DNS", Type: "recursive"},

		// ============================================
		// ACADEMIC & RESEARCH RECURSIVE DNS
		// ============================================
		// Institute for Research in Fundamental Sciences (IPM/IRIPM)
		{Address: "194.225.73.141", Name: "IRIPM Recursive DNS (persia.iranet.ir)", Type: "recursive"},

		// Iran Organization for Science & Technology (IROST)
		{Address: "213.176.123.5", Name: "IROST Recursive DNS", Type: "recursive"},

		// Tehran University of Medical Sciences (TUMS)
		{Address: "194.225.62.80", Name: "TUMS Recursive DNS (ourdns1.tums.ac.ir)", Type: "recursive"},

		// ============================================
		// REGIONAL & MUNICIPAL RECURSIVE DNS
		// ============================================
		// Tehran Municipality ICT Organization
		{Address: "31.24.234.34", Name: "Tehran Municipality DNS (Primary)", Type: "recursive"},
		{Address: "31.24.234.35", Name: "Tehran Municipality DNS (Secondary)", Type: "recursive"},
		{Address: "31.24.234.37", Name: "Tehran Municipality DNS (Tertiary)", Type: "recursive"},

		// Kish Cell Pars (KCP Cloud)
		{Address: "91.245.229.1", Name: "Kish Cell Pars DNS", Type: "recursive"},

		// ============================================
		// OTHER PROVIDERS - RECURSIVE DNS
		// ============================================
		// Hamkaran System
		{Address: "185.187.84.15", Name: "Hamkaran System DNS", Type: "recursive"},

		// Tehran (General/Unspecified Provider)
		{Address: "37.156.145.229", Name: "Tehran Public DNS", Type: "recursive"},
	}
}

// GetDefaultIranianASNs returns a comprehensive list of ALL Iranian ASNs
// Organized by organization/company to include all ASNs for each entity
// This includes main ASNs, subsidiaries, regional networks, and datacenter-specific ASNs
func GetDefaultIranianASNs() []string {
	return []string{
		// ============================================
		// TIC (Telecommunication Infrastructure Company) - tic.ir
		// ============================================
		"AS12880", // TIC (tic.ir) - Telecommunication Infrastructure Company

		// ============================================
		// MOBILE OPERATORS - All ASNs
		// ============================================

		// Mobile Communication Company of Iran (MCCI/Hamrah-e Avval)
		"AS197207", // MCCI - Main mobile network

		// Irancell (MTN Irancell)
		"AS44244", // Irancell - Main mobile network

		// Rightel Communication Service Company
		"AS57218", // Rightel - Main mobile network
		"AS62140", // RIGHTEL-DC - Rightel Data Center services

		// ============================================
		// TCI/ITC GROUP - All ASNs
		// ============================================

		// Iran Telecommunication Company PJS (TCI)
		"AS58224", // TCI - Main backbone/ISP

		// Telecommunication Infrastructure Company (TIC)
		"AS49666", // TIC - Infrastructure backbone

		// ============================================
		// SHATEL GROUP - All ASNs
		// ============================================

		// Aria Shatel PJSC (Shatel)
		"AS31549", // Shatel - Main ISP/broadband

		// ============================================
		// ASIATECH GROUP - All ASNs
		// ============================================

		// Asiatech Data Transmission Company
		"AS43754", // Asiatech - Main ISP/datacenter
		"AS51433", // Asiatech - Additional range/datacenter services

		// ============================================
		// CLOUD & CDN PROVIDERS - All ASNs
		// ============================================

		// Arvan Cloud / Abrarvan (Noyan Abr Arvan Co)
		"AS202468", // Arvan Cloud - Main cloud/CDN/IaaS

		// Respina Networks & Beyond PJSC
		"AS42337", // Respina - Hosting/ISP/CDN

		// Hezardastan Unit Cloud Computing PJSC
		"AS202319", // Hezardastan Cloud - Cloud computing services

		// Hostiran-Network (Noavaran Shabakeh Sabz Mehregan)
		"AS59441", // Hostiran - Hosting/cloud services

		// Khallagh Borhan Market Development (IRCDN)
		"AS8868", // IRCDN - CDN services

		// ============================================
		// GLOBAL CDN & CLOUD PROVIDERS - All ASNs
		// ============================================

		// Cloudflare, Inc. (Global CDN/DNS/WAF provider)
		"AS13335",  // Cloudflare - Main ASN (CLOUDFLARENET)
		"AS14789",  // Cloudflare - Secondary ASN (CLOUDFLARENET)
		"AS202623", // Cloudflare - Core network ASN (CLOUDFLARENET-CORE)
		"AS132892", // Cloudflare - Additional ASN

		// ============================================
		// MAJOR ISPs - All ASNs
		// ============================================

		// Mobinnet Communication Company
		"AS50810", // Mobinnet - ISP/broadband

		// Dadeh Gostar Asr Novin Co (HiWEB)
		"AS56402", // HiWEB - ISP/hosting

		// Parsan Lin Co. PJS / ParsOnline
		"AS16322", // Parsan Lin - ISP/hosting
		"AS58901", // ParsOnline - ISP/datacenter

		// Parvaresh Dadeha Co. (Sabanet/NGS-NedaGostarSaba)
		"AS39501", // Sabanet/NGS - ISP/broadband

		// Afranet
		"AS25184", // Afranet - Hosting/ISP

		// Fanap Telecom
		"AS24631", // Fanap - ISP/telecom

		// IranianNet Communication and Electronic Services Co
		"AS52049", // IranianNet - ISP/broadband

		// Pishgaman Toseeh Ertebatat Co
		"AS49100", // Pishgaman - ISP/network services

		// Tose'h Fanavari Ertebatat Pasargad Arian Co
		"AS206065", // Pasargad Arian - ISP/tech services

		// Ertebatat Sabet Parsian Co
		"AS44400", // Parsian - ISP/broadband

		// ============================================
		// HOSTING & DATACENTER PROVIDERS - All ASNs
		// ============================================

		// Datak Company LLC
		"AS25124", // Datak - Hosting/business services

		// Pardis Fanvari Partak Ltd
		"AS205647", // Pardis Fanvari - Hosting/infrastructure

		// ============================================
		// REGIONAL & MUNICIPAL ISPs - All ASNs
		// ============================================

		// Information Technology Organization of Isfahan Municipality
		"AS56461", // Isfahan Municipality - Municipal network

		// ============================================
		// ACADEMIC & RESEARCH NETWORKS - All ASNs
		// ============================================

		// Institute for Research in Fundamental Sciences (IPM)
		"AS6736", // IRANET-IPM - Research network

		// IsIran (Education/ISP)
		"AS25306", // IsIran - Education network

		// ============================================
		// ADDITIONAL ISPs & NETWORKS
		// ============================================

		// Shabdiz Telecom Network PJSC
		"AS50530", // Shabdiz Telecom - ISP/telecom

		// ============================================
		// ADDITIONAL DATACENTERS & HOSTING PROVIDERS
		// ============================================

		// Mabna (Satcomco)
		"AS49981", // Mabna - ISP/Datacenter

		// ParsPack (Vandad Vira Hooman LLC)
		"AS60631", // ParsPack - Hosting services

		// IranServer (Green Web Samaneh Novin PJSC)
		"AS61173", // IranServer - Datacenter/hosting

		// Iranian Data Center (KEYANA Information Technology Co. Ltd.)
		"AS57067", // Iranian Data Center - Datacenter services

		// ============================================
		// CROSS-BORDER / SUSPICIOUS ASNs
		// ASNs registered outside Iran (Iraq, UAE) but physically operating in Iran
		// or serving Iranian/Iraqi networks with ambiguous infrastructure location
		// ============================================

		// Iraq-Registered ASNs (DMCC = Dubai Multi Commodities Centre)
		"AS199739", // Earthlink-DMCC-IQ - Iraq registered, suspected Iran operations
		"AS50710",  // Earthlink Telecommunications - Iraq ISP with Iran presence
		"AS59692",  // IQWeb FZ-LLC - Iraq web hosting, suspected Iran infrastructure
		"AS203214", // Hulum Almustakbal - Iraq registered

		// UAE-Registered ASNs Operating with Iranian Networks
		"AS57568",  // ARVANCLOUD GLOBAL - Arvan Cloud's global/UAE infrastructure
		"AS208800", // G42 CLOUD - UAE cloud provider with Iran presence
		"AS41268",  // Sesameware FZ-LLC - UAE registered, Iran operations
		"AS60924",  // Orixcom DMCC - UAE registered, suspected Iran infrastructure
		"AS198398", // Symphony Solutions FZ-LLC - UAE, imports BGP from Iran (confirmed)

		// Historical Cross-Border Registration Issues
		"AS41152", // Ertebatat Fara Gostar - Historical UAE registration, now Iran

		// Additional regional ISPs and networks
		// Note: Many organizations may have additional ASNs for subsidiaries,
		// regional operations, or specific services that are not publicly well-documented.
		// This list focuses on active, announced ASNs visible in BGP routing tables.
		// Note: Tehran Internet Exchange (TIX), ITRC Internet Data Center, HelmaGostar,
		// AsrTelcom, and Mabna Cloud were not found with publicly routable ASNs or domains.
	}
}

// GetASNName returns a readable name for an ASN
func GetASNName(asn string) string {
	asnNames := map[string]string{
		// Mobile Operators
		"AS197207": "MCCI (Hamrah-e Avval)",
		"AS44244":  "Irancell (MTN Irancell)",
		"AS57218":  "Rightel",
		"AS62140":  "Rightel Data Center",

		// TIC (Telecommunication Infrastructure Company) - tic.ir
		"AS12880": "TIC (tic.ir)",

		// TCI/ITC Group
		"AS58224": "TCI (Iran Telecommunication Company)",
		"AS49666": "TIC (Telecommunication Infrastructure Company)",

		// Shatel Group
		"AS31549": "Shatel (Aria Shatel)",

		// Asiatech Group
		"AS43754": "Asiatech",
		"AS51433": "Asiatech (Additional)",

		// Cloud & CDN Providers (Iranian)
		"AS202468": "Arvan Cloud (Abrarvan)",
		"AS42337":  "Respina Networks",
		"AS202319": "Hezardastan Cloud",
		"AS59441":  "Hostiran",
		"AS8868":   "IRCDN",

		// Global CDN & Cloud Providers
		"AS13335":  "Cloudflare (Main)",
		"AS14789":  "Cloudflare (Secondary)",
		"AS202623": "Cloudflare (Core)",
		"AS132892": "Cloudflare (Additional)",

		// Major ISPs
		"AS50810":  "Mobinnet",
		"AS56402":  "HiWEB",
		"AS16322":  "Parsan Lin",
		"AS58901":  "ParsOnline",
		"AS39501":  "Sabanet/NGS",
		"AS25184":  "Afranet",
		"AS24631":  "Fanap Telecom",
		"AS52049":  "IranianNet",
		"AS49100":  "Pishgaman",
		"AS206065": "Pasargad Arian",
		"AS44400":  "Parsian",
		"AS50530":  "Shabdiz Telecom",

		// Hosting & Datacenter Providers
		"AS25124":  "Datak",
		"AS205647": "Pardis Fanvari",

		// Regional & Municipal ISPs
		"AS56461": "Isfahan Municipality",

		// Academic & Research Networks
		"AS6736":  "IPM (Institute for Research in Fundamental Sciences)",
		"AS25306": "IsIran",

		// Additional Datacenters & Hosting Providers
		"AS49981": "Mabna (Satcomco)",
		"AS60631": "ParsPack",
		"AS61173": "IranServer",
		"AS57067": "Iranian Data Center",
	}

	if name, exists := asnNames[asn]; exists {
		return name
	}
	return "Unknown"
}

