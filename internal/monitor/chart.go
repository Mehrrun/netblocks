package monitor

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/netblocks/netblocks/internal/models"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// GenerateTrafficChart generates a PNG chart of Iran traffic (24h)
func GenerateTrafficChart(data *TrafficData) (*bytes.Buffer, error) {
	if data == nil || len(data.Trend24h) == 0 {
		return nil, fmt.Errorf("no traffic data available")
	}
	unit := displayUnit(data.Unit, data.Trend24h)
	return renderAbsoluteLineChart(
		data.Trend24h,
		data.Timestamps,
		data.Status,
		unit,
		fmt.Sprintf("Iran Internet Traffic — Volume Index (24h)"),
		"Hours Ago",
		true,
	)
}

// GenerateTrafficChart7d generates a PNG chart of Iran traffic (7d)
func GenerateTrafficChart7d(data *TrafficData) (*bytes.Buffer, error) {
	if data == nil || len(data.Trend7d) == 0 {
		return nil, fmt.Errorf("no 7d traffic data available")
	}
	unit := displayUnit(data.Unit, data.Trend7d)
	return renderAbsoluteLineChart(
		data.Trend7d,
		data.Timestamps7d,
		data.Status,
		unit,
		fmt.Sprintf("Iran Internet Traffic — Volume Index (7d)"),
		"Days Ago",
		false,
	)
}

// displayUnit forces index labeling when values are in the 0–1 Radar index range
func displayUnit(unit string, values []float64) string {
	if isIndexUnit(unit) {
		return "index"
	}
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal > 0 && maxVal <= 1.5 {
		return "index"
	}
	return unit
}

func renderAbsoluteLineChart(values []float64, timestamps []time.Time, status, unit, title, xName string, hourly bool) (*bytes.Buffer, error) {
	n := len(values)
	xValues := make([]float64, n)
	yValues := make([]float64, n)
	copy(yValues, values)

	// Chronological left→right (oldest → newest)
	for i := 0; i < n; i++ {
		xValues[i] = float64(i)
	}

	maxY := 0.0
	for _, v := range yValues {
		if v > maxY {
			maxY = v
		}
	}
	if maxY <= 0 {
		maxY = 1
	}
	yMax := maxY * 1.12

	lineColor := statusLineColor(status)
	yLabel := axisLabelForUnit(unit)
	points := n

	graph := chart.Chart{
		Width:  1000,
		Height: 450,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    55,
				Left:   25,
				Right:  25,
				Bottom: 25,
			},
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255},
		},
		XAxis: chart.XAxis{
			Name: xName,
			ValueFormatter: func(v interface{}) string {
				vf, ok := v.(float64)
				if !ok {
					return ""
				}
				ago := float64(points-1) - vf
				if ago < 0 {
					ago = 0
				}
				if hourly {
					return fmt.Sprintf("%.0fh", ago)
				}
				return fmt.Sprintf("%.1fd", ago/24.0)
			},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: float64(n - 1),
			},
		},
		YAxis: chart.YAxis{
			Name: yLabel,
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: yMax,
			},
			ValueFormatter: func(v interface{}) string {
				vf, ok := v.(float64)
				if !ok {
					return ""
				}
				return formatAxisValue(vf, unit)
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Traffic",
				XValues: xValues,
				YValues: yValues,
				Style: chart.Style{
					StrokeColor: lineColor,
					StrokeWidth: 2.5,
				},
			},
		},
	}

	graph.Title = title
	graph.TitleStyle = chart.Style{FontSize: 15}
	_ = timestamps

	buffer := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.PNG, buffer); err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}
	return buffer, nil
}

func statusLineColor(status string) drawing.Color {
	switch status {
	case "Normal":
		return drawing.Color{R: 46, G: 125, B: 50, A: 255}
	case "Degraded":
		return drawing.Color{R: 245, G: 166, B: 35, A: 255}
	case "Throttled":
		return drawing.Color{R: 230, G: 126, B: 34, A: 255}
	case "Shutdown":
		return drawing.Color{R: 198, G: 40, B: 40, A: 255}
	default:
		return drawing.Color{R: 25, G: 118, B: 210, A: 255}
	}
}

func axisLabelForUnit(unit string) string {
	u := strings.ToLower(unit)
	switch {
	case isIndexUnit(u):
		return "Volume Index (0–1)"
	case strings.Contains(u, "byte"):
		return "Traffic Volume"
	case strings.Contains(u, "request"):
		return "HTTP Requests"
	default:
		if unit != "" {
			return "Traffic (" + unit + ")"
		}
		return "Traffic Volume"
	}
}

func formatAxisValue(v float64, unit string) string {
	u := strings.ToLower(unit)
	if isIndexUnit(u) {
		return fmt.Sprintf("%.2f", v)
	}
	if strings.Contains(u, "byte") {
		return formatBytes(v)
	}
	return formatCompact(v)
}

// FormatTrafficStatus formats traffic data for photo captions
func FormatTrafficStatus(data *models.TrafficData) string {
	if data == nil {
		return "❌ Traffic data unavailable"
	}

	timeStr := formatDuration(time.Since(data.LastUpdate))
	current := formatAbsoluteValue(data.CurrentLevel, data.Unit)
	baseline := formatAbsoluteValue(data.Baseline, data.Unit)

	heading := "*Iran Traffic — Volume Index*"
	howToRead := "\n📖 *How to read:* Y-axis is Cloudflare’s *volume index* (~`0`–`1`), not bytes. Higher = more traffic; compare shape & % change."
	if !isIndexUnit(data.Unit) {
		maxVal := data.CurrentLevel
		for _, v := range data.Trend24h {
			if v > maxVal {
				maxVal = v
			}
		}
		for _, v := range data.Trend7d {
			if v > maxVal {
				maxVal = v
			}
		}
		if maxVal > 1.5 {
			heading = "*Iran Traffic*"
			howToRead = ""
		}
	}

	statusText := fmt.Sprintf(
		"%s %s\n"+
			"📶 *Current:* `%s`\n"+
			"📐 *Baseline:* `%s`\n"+
			"📈 *Change:* `%+.1f%%`\n"+
			"📊 *Status:* %s\n"+
			"⏱ *Updated:* %s ago%s",
		data.StatusEmoji,
		heading,
		current,
		baseline,
		data.ChangePercent,
		data.Status,
		timeStr,
		howToRead,
	)

	if data.Status == "Shutdown" || data.Status == "Throttled" {
		statusText += "\n\n⚠️ *MAJOR DISRUPTION DETECTED*"
	}
	return statusText
}

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

// GenerateASNTrafficChart generates a high-contrast bar chart for ASN traffic share
func GenerateASNTrafficChart(data []*models.ASTrafficData) (*bytes.Buffer, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no ASN traffic data available")
	}

	maxItems := 20
	if len(data) > maxItems {
		data = data[:maxItems]
	}

	barValues := make([]chart.Value, len(data))
	maxPercentage := 0.0
	for i, item := range data {
		percentage := item.Percentage
		if percentage > maxPercentage {
			maxPercentage = percentage
		}

		label := fmt.Sprintf("%s - %s", item.ASN, item.Name)
		if len(label) > 36 {
			maxNameLen := 36 - len(item.ASN) - 3
			if maxNameLen > 0 && len(item.Name) > maxNameLen {
				label = fmt.Sprintf("%s - %s…", item.ASN, item.Name[:maxNameLen])
			} else {
				label = item.ASN
			}
		}

		// Darker blue with distinct stroke for visibility on white
		barColor := drawing.Color{R: 30, G: 136, B: 229, A: 255}
		stroke := drawing.Color{R: 13, G: 71, B: 161, A: 255}

		barValues[i] = chart.Value{
			Label: label,
			Value: percentage,
			Style: chart.Style{
				FillColor:   barColor,
				StrokeColor: stroke,
				StrokeWidth: 1,
			},
		}
	}

	if maxPercentage <= 0 {
		maxPercentage = 1
	}

	graph := chart.BarChart{
		Width:  1200,
		Height: 650,
		Title:  fmt.Sprintf("Top %d Iranian ASNs by Traffic Share", len(data)),
		TitleStyle: chart.Style{
			FontSize: 16,
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:    60,
				Left:   90,
				Right:  25,
				Bottom: 45,
			},
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255},
		},
		BarWidth: 28,
		XAxis: chart.Style{
			FontSize: 9,
		},
		YAxis: chart.YAxis{
			Name:      "Traffic Share (%)",
			NameStyle: chart.Style{FontSize: 12},
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: math.Min(100, maxPercentage*1.15),
			},
			ValueFormatter: func(v interface{}) string {
				if vf, ok := v.(float64); ok {
					return fmt.Sprintf("%.1f%%", vf)
				}
				return ""
			},
		},
		Bars: barValues,
	}

	buffer := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.PNG, buffer); err != nil {
		return nil, fmt.Errorf("failed to render ASN traffic bar chart: %w", err)
	}
	return buffer, nil
}
