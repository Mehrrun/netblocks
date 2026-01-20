package monitor

import (
	"bytes"
	"fmt"
	"time"

	"github.com/netblocks/netblocks/internal/models"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// GenerateTrafficChart generates a PNG chart image from traffic data
func GenerateTrafficChart(data *TrafficData) (*bytes.Buffer, error) {
	if data == nil || len(data.Trend24h) == 0 {
		return nil, fmt.Errorf("no traffic data available")
	}

	// Prepare X values (hours ago)
	xValues := make([]float64, len(data.Trend24h))
	for i := range xValues {
		xValues[i] = float64(len(data.Trend24h) - i - 1) // Hours ago
	}

	// Reverse for chronological order (oldest to newest)
	for i, j := 0, len(xValues)-1; i < j; i, j = i+1, j-1 {
		xValues[i], xValues[j] = xValues[j], xValues[i]
	}

	yValues := make([]float64, len(data.Trend24h))
	copy(yValues, data.Trend24h)
	for i, j := 0, len(yValues)-1; i < j; i, j = i+1, j-1 {
		yValues[i], yValues[j] = yValues[j], yValues[i]
	}

	// Determine line color based on status
	var lineColor drawing.Color
	switch data.Status {
	case "Normal":
		lineColor = drawing.Color{R: 76, G: 175, B: 80, A: 255} // Green
	case "Degraded":
		lineColor = drawing.Color{R: 255, G: 193, B: 7, A: 255} // Yellow
	case "Throttled":
		lineColor = drawing.Color{R: 255, G: 152, B: 0, A: 255} // Orange
	case "Shutdown":
		lineColor = drawing.Color{R: 244, G: 67, B: 54, A: 255} // Red
	default:
		lineColor = chart.ColorBlue
	}

	// Create the chart
	graph := chart.Chart{
		Width:  800,
		Height: 400,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255}, // White background
		},
		XAxis: chart.XAxis{
			Name:      "Hours Ago",
			NameStyle: chart.Style{},
			Style:     chart.Style{},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					return fmt.Sprintf("%.0fh", vf)
				}
				return ""
			},
		},
		YAxis: chart.YAxis{
			Name:      "Traffic Level (%)",
			NameStyle: chart.Style{},
			Style:     chart.Style{},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: 100,
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Traffic",
				XValues: xValues,
				YValues: yValues,
				Style: chart.Style{
					StrokeColor: lineColor,
					StrokeWidth: 3,
				},
			},
		},
	}

	// Add title
	graph.Title = "Iran Internet Traffic (Last 24h)"
	graph.TitleStyle = chart.Style{
		FontSize: 16,
	}

	// Render to buffer
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return buffer, nil
}

// FormatTrafficStatus formats traffic data for text display
func FormatTrafficStatus(data *models.TrafficData) string {
	if data == nil {
		return "❌ Traffic data unavailable"
	}

	timeSince := time.Since(data.LastUpdate)
	timeStr := formatDuration(timeSince)

	statusText := fmt.Sprintf(
		"%s *Traffic Level:* %.1f%%\n"+
			"📈 *Change:* %+.1f%%\n"+
			"📊 *Status:* %s\n"+
			"⏱ *Updated:* %s ago",
		data.StatusEmoji,
		data.CurrentLevel,
		data.ChangePercent,
		data.Status,
		timeStr,
	)

	if data.Status == "Shutdown" || data.Status == "Throttled" {
		statusText += "\n\n⚠️ *MAJOR DISRUPTION DETECTED*"
	}

	return statusText
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d secs", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d mins", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

// GenerateASNTrafficChart generates a bar chart visualization for ASN traffic data
// Shows top 10 Iranian ASNs with their names and current bandwidth (independent bars)
func GenerateASNTrafficChart(data []*models.ASTrafficData) (*bytes.Buffer, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no ASN traffic data available")
	}

	// Limit to top 10 ASNs (already sorted by traffic volume)
	maxItems := 10
	if len(data) > maxItems {
		data = data[:maxItems]
	}

	// Prepare data for bar chart - use bandwidth (TrafficVolume) instead of percentage
	barValues := make([]chart.Value, len(data))
	maxBandwidth := 0.0
	for i, item := range data {
		// Use TrafficVolume as the bandwidth value
		bandwidth := item.TrafficVolume
		if bandwidth > maxBandwidth {
			maxBandwidth = bandwidth
		}
		
		// Create label: "AS12345 - Name" to show both ASN and name
		label := fmt.Sprintf("%s - %s", item.ASN, item.Name)
		if len(label) > 40 {
			// Truncate long names but keep ASN visible
			maxNameLen := 40 - len(item.ASN) - 3 // Reserve space for ASN, " - ", and "..."
			if maxNameLen > 0 {
				label = fmt.Sprintf("%s - %s...", item.ASN, item.Name[:maxNameLen])
			} else {
				// If ASN itself is too long, just use ASN
				label = item.ASN
			}
		}
		
		// Use light blue color for all bars (white-ish but a bit blue)
		// Light blue: RGB(173, 216, 230) or similar - slightly lighter
		barColor := drawing.Color{R: 176, G: 224, B: 230, A: 255} // Light blue (PowderBlue)
		
		barValues[i] = chart.Value{
			Label: label,
			Value: percentage, // This is a percentage value from the API
			Style: chart.Style{
				FillColor:   barColor,
				StrokeColor: barColor,
				StrokeWidth: 1,
			},
		}
	}

	// Create bar chart
	graph := chart.BarChart{
		Width:  1200, // Wider to accommodate ASN names
		Height: 600,  // Taller for better readability
		Title:  fmt.Sprintf("Top %d Iranian ASNs by Traffic Share", len(data)),
		TitleStyle: chart.Style{
			FontSize: 18,
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:    60,
				Left:   100, // More left padding for ASN names
				Right:  20,
				Bottom: 40,
			},
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255}, // White background
		},
		BarWidth: 40, // Width of each bar
		XAxis: chart.Style{
			FontSize: 10,
		},
		YAxis: chart.YAxis{
			Name:      "Traffic Share (%)",
			NameStyle: chart.Style{FontSize: 14},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: maxPercentage * 1.1, // Add 10% padding (values are already percentages)
			},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					// Values are already percentages from Cloudflare API
					// Format as percentage with 1 decimal place
					return fmt.Sprintf("%.1f%%", vf)
				}
				return ""
			},
		},
		Bars: barValues,
	}

	// Render to buffer
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render ASN traffic bar chart: %w", err)
	}

	return buffer, nil
}

