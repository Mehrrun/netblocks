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

// GenerateASNTrafficChart generates a vertical bar chart for ASN traffic data
func GenerateASNTrafficChart(data []*models.ASTrafficData) (*bytes.Buffer, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no ASN traffic data available")
	}

	// Limit to top 20 ASNs (already sorted by traffic volume)
	maxBars := 20
	if len(data) > maxBars {
		data = data[:maxBars]
	}

	// Prepare data for chart - use vertical bar chart (standard approach)
	xValues := make([]float64, len(data))
	yValues := make([]float64, len(data))
	labels := make([]string, len(data))
	colors := make([]drawing.Color, len(data))

	// Get max percentage for Y-axis range
	maxPercentage := 0.0
	for _, item := range data {
		if item.Percentage > maxPercentage {
			maxPercentage = item.Percentage
		}
	}

	// Prepare values (use percentage for better readability)
	for i, item := range data {
		// X-axis: index position
		xValues[i] = float64(i)
		
		// Y-axis: percentage value
		yValues[i] = item.Percentage
		
		// Create label: ASN name (truncate if too long for readability)
		label := item.Name
		if len(label) > 25 {
			label = label[:22] + "..."
		}
		// Use shorter label with percentage for display
		labels[i] = label

		// Color-code bars based on status
		switch item.Status {
		case "High":
			colors[i] = drawing.Color{R: 76, G: 175, B: 80, A: 255} // Green
		case "Medium":
			colors[i] = drawing.Color{R: 255, G: 193, B: 7, A: 255} // Yellow
		case "Low":
			colors[i] = drawing.Color{R: 255, G: 152, B: 0, A: 255} // Orange
		default:
			colors[i] = drawing.Color{R: 200, G: 200, B: 200, A: 255} // Gray
		}
	}

	// Create vertical bar chart - using stacked bars approach
	// We'll create bars by drawing filled rectangles using Value series
	graph := chart.Chart{
		Width:  1200,
		Height: 700, // Taller to accommodate 20 bars and labels
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  30,
				Bottom: 200, // More bottom padding for rotated ASN labels
			},
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255}, // White background
		},
		XAxis: chart.XAxis{
			Name:      "ASN",
			NameStyle: chart.Style{},
			Style:     chart.Style{},
			ValueFormatter: func(v interface{}) string {
				if idx, ok := v.(float64); ok {
					i := int(idx)
					if i >= 0 && i < len(labels) {
						return labels[i]
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
				Max: maxPercentage * 1.15, // Add 15% padding for better visibility
			},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					return fmt.Sprintf("%.1f%%", vf)
				}
				return ""
			},
		},
	}

	// Create bars using histogram-style visualization
	// Each bar is created by drawing a filled area from 0 to value
	for i := range xValues {
		// Create a bar by drawing from x to x+barWidth, from 0 to yValue
		barWidth := 0.6 // Width of each bar
		barSeries := chart.ContinuousSeries{
			XValues: []float64{xValues[i], xValues[i] + barWidth, xValues[i] + barWidth, xValues[i]},
			YValues: []float64{0, 0, yValues[i], yValues[i]}, // Rectangle: bottom-left, bottom-right, top-right, top-left
			Style: chart.Style{
				StrokeColor:     colors[i],
				FillColor:       colors[i],
				StrokeWidth:     2,
				DotWidth:        0,
			},
		}
		graph.Series = append(graph.Series, barSeries)
	}

	// Add title
	graph.Title = fmt.Sprintf("Top %d Iranian ASNs by Traffic (Current)", len(data))
	graph.TitleStyle = chart.Style{
		FontSize: 16,
	}

	// Render to buffer
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render ASN traffic chart: %w", err)
	}

	return buffer, nil
}

