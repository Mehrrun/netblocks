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

// GenerateASNTrafficChart generates a line chart style visualization for ASN traffic data
// Follows the exact same pattern as GenerateTrafficChart for consistency
func GenerateASNTrafficChart(data []*models.ASTrafficData) (*bytes.Buffer, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no ASN traffic data available")
	}

	// Limit to top 20 ASNs (already sorted by traffic volume)
	maxItems := 20
	if len(data) > maxItems {
		data = data[:maxItems]
	}

	// Prepare X values (ASN index) - similar to working chart pattern
	xValues := make([]float64, len(data))
	for i := range xValues {
		xValues[i] = float64(i) // ASN index: 0, 1, 2, ...
	}

	// Prepare Y values (traffic percentage) - similar to working chart pattern
	yValues := make([]float64, len(data))
	maxPercentage := 0.0
	for i, item := range data {
		yValues[i] = item.Percentage
		if item.Percentage > maxPercentage {
			maxPercentage = item.Percentage
		}
	}

	// Determine line color based on average status or use a default
	// Use blue as default (matching working chart pattern)
	var lineColor drawing.Color = chart.ColorBlue
	if len(data) > 0 {
		// Use color of top ASN as line color
		switch data[0].Status {
		case "High":
			lineColor = drawing.Color{R: 76, G: 175, B: 80, A: 255} // Green
		case "Medium":
			lineColor = drawing.Color{R: 255, G: 193, B: 7, A: 255} // Yellow
		case "Low":
			lineColor = drawing.Color{R: 255, G: 152, B: 0, A: 255} // Orange
		default:
			lineColor = chart.ColorBlue
		}
	}

	// Create the chart - following exact same pattern as GenerateTrafficChart
	graph := chart.Chart{
		Width:  800,  // Same width as working chart
		Height: 400,  // Same height as working chart
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
			Name:      "ASN (Top 20)",
			NameStyle: chart.Style{},
			Style:     chart.Style{},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					idx := int(vf)
					if idx >= 0 && idx < len(data) {
						// Show ASN number instead of index
						return fmt.Sprintf("%d", idx+1)
					}
				}
				return ""
			},
		},
		YAxis: chart.YAxis{
			Name:      "Traffic Volume (%)",
			NameStyle: chart.Style{},
			Style:     chart.Style{},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: maxPercentage * 1.1, // Add 10% padding
			},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					return fmt.Sprintf("%.1f%%", vf)
				}
				return ""
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "ASN Traffic",
				XValues: xValues,
				YValues: yValues,
				Style: chart.Style{
					StrokeColor: lineColor,
					StrokeWidth: 3,
					DotWidth:    5, // Add visible dots at each ASN point
					DotColor:    lineColor,
				},
			},
		},
	}

	// Add title - similar to working chart
	graph.Title = fmt.Sprintf("Top %d Iranian ASNs by Traffic (Current)", len(data))
	graph.TitleStyle = chart.Style{
		FontSize: 16,
	}

	// Render to buffer - same pattern as working chart
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render ASN traffic chart: %w", err)
	}

	return buffer, nil
}

