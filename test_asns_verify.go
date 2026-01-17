package main

import (
	"fmt"
	"github.com/netblocks/netblocks/internal/config"
)

func main() {
	asns := config.GetDefaultIranianASNs()
	seen := make(map[string]bool)
	duplicates := []string{}
	invalidFormat := []string{}

	for _, asn := range asns {
		// Check for duplicates
		if seen[asn] {
			duplicates = append(duplicates, asn)
		} else {
			seen[asn] = true
		}

		// Check format (should start with "AS" followed by digits)
		if len(asn) < 3 || asn[:2] != "AS" {
			invalidFormat = append(invalidFormat, asn)
		}
	}

	fmt.Printf("Total ASNs: %d\n", len(asns))
	fmt.Printf("Unique ASNs: %d\n", len(seen))

	if len(duplicates) > 0 {
		fmt.Printf("\n❌ Duplicates found: %v\n", duplicates)
	} else {
		fmt.Println("\n✓ No duplicates found")
	}

	if len(invalidFormat) > 0 {
		fmt.Printf("\n❌ Invalid format: %v\n", invalidFormat)
	} else {
		fmt.Println("✓ All ASNs have correct format (AS####)")
	}

	// Show organization grouping
	fmt.Println("\n=== Organization Summary ===")
	orgCounts := map[string]int{
		"Mobile Operators": 3,
		"TCI/ITC Group": 3,
		"Shatel": 1,
		"Asiatech": 2,
		"Cloud/CDN": 5,
		"Major ISPs": 10,
		"Hosting/Datacenter": 2,
		"Regional/Municipal": 1,
		"Academic/Research": 2,
		"Additional": 1,
	}
	
	total := 0
	for org, count := range orgCounts {
		fmt.Printf("%s: %d ASNs\n", org, count)
		total += count
	}
	fmt.Printf("\nTotal: %d ASNs\n", total)
}

