package reporting

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// HTMLReportOptions configures HTML rendering parameters.
type HTMLReportOptions struct {
	Title          string
	History        []*model.SystemSnapshot
	IncludeCharts  bool
	IncludeRawJSON bool
}

// GenerateHTML renders a standalone HTML dashboard report.
func GenerateHTML(data *model.ReportData, opts HTMLReportOptions) ([]byte, error) {
	if data == nil {
		data = &model.ReportData{
			Title:       "Watchdog System Report",
			GeneratedAt: time.Now(),
		}
	}

	title := data.Title
	if opts.Title != "" {
		title = opts.Title
	}

	tmplData := struct {
		Title          string
		Data           *model.ReportData
		GeneratedAt    string
		CPUUsage       float64
		MemoryUsage    float64
		DiskUsage      float64
		SwapUsage      float64
		HealthClass    string
		HealthLabel    string
		CPUSparkline   template.HTML
		MemSparkline   template.HTML
		NetRxSparkline template.HTML
	}{
		Title:       title,
		Data:        data,
		GeneratedAt: data.GeneratedAt.Format("2006-01-02 15:04:05 MST"),
		CPUUsage:    data.CPU.OverallUsage,
		MemoryUsage: data.Memory.UsedPercent,
		DiskUsage:   data.Disk.UsedPercent,
		SwapUsage:   data.Memory.SwapUsedPercent,
		HealthClass: "healthy",
		HealthLabel: "HEALTHY",
	}

	if data.Diagnostics != nil {
		switch data.Diagnostics.OverallStatus {
		case model.StatusFail:
			tmplData.HealthClass = "critical"
			tmplData.HealthLabel = "CRITICAL"
		case model.StatusWarning:
			tmplData.HealthClass = "warning"
			tmplData.HealthLabel = "WARNING"
		}
	}

	// Generate SVG sparklines if historical snapshots exist
	if opts.IncludeCharts && len(opts.History) > 1 {
		var cpuPoints []float64
		var memPoints []float64
		var netRxPoints []float64

		for _, s := range opts.History {
			if s == nil {
				continue
			}
			if s.CPU != nil {
				cpuPoints = append(cpuPoints, s.CPU.OverallUsage)
			}
			if s.Memory != nil {
				memPoints = append(memPoints, s.Memory.UsedPercent)
			}
			if s.Network != nil {
				netRxPoints = append(netRxPoints, s.Network.TotalRxRate)
			}
		}

		tmplData.CPUSparkline = template.HTML(renderSVGSparkline(cpuPoints, 320, 60, "#3b82f6"))
		tmplData.MemSparkline = template.HTML(renderSVGSparkline(memPoints, 320, 60, "#8b5cf6"))
		tmplData.NetRxSparkline = template.HTML(renderSVGSparkline(netRxPoints, 320, 60, "#10b981"))
	}

	t, err := template.New("report").Funcs(template.FuncMap{
		"formatBytes": formatBytes,
		"formatRate":  func(f float64) string { return formatBytes(uint64(f)) },
		"formatFloat": func(v any) string {
			switch val := v.(type) {
			case float64:
				return fmt.Sprintf("%.1f", val)
			case float32:
				return fmt.Sprintf("%.1f", val)
			default:
				return fmt.Sprintf("%v", val)
			}
		},
		"formatDuration": func(d time.Duration) string {
			days := int(d.Hours()) / 24
			hours := int(d.Hours()) % 24
			mins := int(d.Minutes()) % 60
			if days > 0 {
				return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
			}
			return fmt.Sprintf("%dh %dm", hours, mins)
		},
		"truncate": truncate,
		"severityClass": func(sev model.Severity) string {
			switch sev {
			case model.SeverityCritical:
				return "badge-critical"
			case model.SeverityWarning:
				return "badge-warning"
			default:
				return "badge-info"
			}
		},
		"statusClass": func(st model.DiagnosticStatus) string {
			switch st {
			case model.StatusFail:
				return "badge-critical"
			case model.StatusWarning:
				return "badge-warning"
			default:
				return "badge-success"
			}
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := t.Execute(buf, tmplData); err != nil {
		return nil, fmt.Errorf("failed to render HTML template: %w", err)
	}

	return buf.Bytes(), nil
}

func renderSVGSparkline(points []float64, width, height int, strokeColor string) string {
	if len(points) < 2 {
		return ""
	}

	minVal, maxVal := points[0], points[0]
	for _, p := range points {
		if p < minVal {
			minVal = p
		}
		if p > maxVal {
			maxVal = p
		}
	}

	valRange := maxVal - minVal
	if valRange <= 0 {
		valRange = 1.0
	}

	stepX := float64(width) / float64(len(points)-1)
	var pathPoints []string

	for i, val := range points {
		x := float64(i) * stepX
		normY := (val - minVal) / valRange
		y := float64(height) - (normY * float64(height-10)) - 5 // 5px padding
		pathPoints = append(pathPoints, fmt.Sprintf("%.1f,%.1f", x, y))
	}

	pathD := "M " + strings.Join(pathPoints, " L ")

	return fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" class="sparkline">
  <path d="%s" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
</svg>`, width, height, width, height, html.EscapeString(pathD), strokeColor)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} - {{.Data.Host.Hostname}}</title>
  <style>
    :root {
      --bg-primary: #0f172a;
      --bg-secondary: #1e293b;
      --bg-card: #1e293b;
      --border-color: #334155;
      --text-primary: #f8fafc;
      --text-secondary: #94a3b8;
      --text-muted: #64748b;
      --accent-blue: #3b82f6;
      --accent-purple: #8b5cf6;
      --accent-green: #10b981;
      --accent-yellow: #f59e0b;
      --accent-red: #ef4444;
      --card-radius: 12px;
    }

    [data-theme="light"] {
      --bg-primary: #f8fafc;
      --bg-secondary: #ffffff;
      --bg-card: #ffffff;
      --border-color: #e2e8f0;
      --text-primary: #0f172a;
      --text-secondary: #475569;
      --text-muted: #94a3b8;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      background-color: var(--bg-primary);
      color: var(--text-primary);
      line-height: 1.5;
      padding: 24px;
      transition: background-color 0.2s ease, color 0.2s ease;
    }

    .container { max-width: 1280px; margin: 0 auto; }

    /* Header */
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 24px;
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: var(--card-radius);
      margin-bottom: 24px;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
    }
    .header-left h1 { font-size: 24px; font-weight: 700; display: flex; align-items: center; gap: 12px; }
    .header-subtitle { color: var(--text-secondary); font-size: 14px; margin-top: 4px; }
    .header-right { display: flex; align-items: center; gap: 16px; }

    /* Health Badge */
    .health-badge {
      display: inline-flex;
      align-items: center;
      padding: 6px 14px;
      border-radius: 9999px;
      font-size: 13px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .health-badge.healthy { background: rgba(16, 185, 129, 0.15); color: var(--accent-green); border: 1px solid var(--accent-green); }
    .health-badge.warning { background: rgba(245, 158, 11, 0.15); color: var(--accent-yellow); border: 1px solid var(--accent-yellow); }
    .health-badge.critical { background: rgba(239, 68, 68, 0.15); color: var(--accent-red); border: 1px solid var(--accent-red); }

    /* Theme toggle */
    .theme-toggle {
      background: var(--bg-secondary);
      border: 1px solid var(--border-color);
      color: var(--text-primary);
      padding: 8px 14px;
      border-radius: 8px;
      cursor: pointer;
      font-size: 13px;
      font-weight: 500;
    }

    /* Grid Layout */
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; margin-bottom: 24px; }

    /* Metric Cards */
    .card {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: var(--card-radius);
      padding: 20px;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05);
    }
    .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
    .card-title { font-size: 14px; font-weight: 600; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.05em; }
    .card-value { font-size: 32px; font-weight: 800; color: var(--text-primary); }
    .card-subtext { font-size: 13px; color: var(--text-muted); margin-top: 6px; }

    /* Progress Bar */
    .progress-bar-container {
      width: 100%;
      height: 8px;
      background: var(--border-color);
      border-radius: 9999px;
      overflow: hidden;
      margin: 12px 0 6px 0;
    }
    .progress-bar {
      height: 100%;
      border-radius: 9999px;
      transition: width 0.4s ease;
    }
    .bg-green { background-color: var(--accent-green); }
    .bg-yellow { background-color: var(--accent-yellow); }
    .bg-red { background-color: var(--accent-red); }
    .bg-blue { background-color: var(--accent-blue); }

    /* Table Styles */
    .section-card {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: var(--card-radius);
      padding: 24px;
      margin-bottom: 24px;
    }
    .section-title { font-size: 18px; font-weight: 700; margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
    .table-container { width: 100%; overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; text-align: left; font-size: 14px; }
    th { padding: 12px 16px; background: rgba(0, 0, 0, 0.1); color: var(--text-secondary); font-weight: 600; border-bottom: 1px solid var(--border-color); }
    td { padding: 12px 16px; border-bottom: 1px solid var(--border-color); color: var(--text-primary); }
    tr:last-child td { border-bottom: none; }
    tr:hover td { background: rgba(255, 255, 255, 0.02); }

    /* Badges */
    .badge {
      display: inline-block;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
    }
    .badge-success { background: rgba(16, 185, 129, 0.15); color: var(--accent-green); }
    .badge-warning { background: rgba(245, 158, 11, 0.15); color: var(--accent-yellow); }
    .badge-critical { background: rgba(239, 68, 68, 0.15); color: var(--accent-red); }
    .badge-info { background: rgba(59, 130, 246, 0.15); color: var(--accent-blue); }

    .sparkline { width: 100%; height: 60px; margin-top: 10px; }
    .code-tag { font-family: monospace; font-size: 12px; background: var(--bg-secondary); padding: 2px 6px; border-radius: 4px; }

    /* Footer */
    .footer { text-align: center; color: var(--text-muted); font-size: 13px; margin-top: 40px; padding-bottom: 20px; }
  </style>
</head>
<body>
  <div class="container">
    <!-- Header -->
    <div class="header">
      <div class="header-left">
        <h1>
          <span>🛡️ Watchdog Report</span>
          <span class="health-badge {{.HealthClass}}">{{.HealthLabel}}</span>
        </h1>
        <div class="header-subtitle">
          Host: <strong>{{.Data.Host.Hostname}}</strong> ({{.Data.Host.OS}} {{.Data.Host.Platform}} {{.Data.Host.PlatformVersion}}) |
          Uptime: {{formatDuration .Data.Host.Uptime}} |
          Generated: {{.GeneratedAt}}
        </div>
      </div>
      <div class="header-right">
        <button class="theme-toggle" onclick="toggleTheme()">🌓 Toggle Theme</button>
      </div>
    </div>

    <!-- Overview Grid -->
    <div class="grid">
      <!-- CPU -->
      <div class="card">
        <div class="card-header">
          <span class="card-title">CPU Usage</span>
          <span>⚡ {{.Data.CPU.LogicalCores}} Cores</span>
        </div>
        <div class="card-value">{{formatFloat .CPUUsage}}%</div>
        <div class="progress-bar-container">
          <div class="progress-bar {{if ge .CPUUsage 85.0}}bg-red{{else if ge .CPUUsage 70.0}}bg-yellow{{else}}bg-blue{{end}}" style="width: {{.CPUUsage}}%;"></div>
        </div>
        <div class="card-subtext">Load: {{formatFloat .Data.CPU.LoadAverage.Load1}} / {{formatFloat .Data.CPU.LoadAverage.Load5}} / {{formatFloat .Data.CPU.LoadAverage.Load15}}</div>
        {{if .CPUSparkline}}{{.CPUSparkline}}{{end}}
      </div>

      <!-- Memory -->
      <div class="card">
        <div class="card-header">
          <span class="card-title">Memory Usage</span>
          <span>🧠 {{formatBytes .Data.Memory.TotalBytes}}</span>
        </div>
        <div class="card-value">{{formatFloat .MemoryUsage}}%</div>
        <div class="progress-bar-container">
          <div class="progress-bar {{if ge .MemoryUsage 85.0}}bg-red{{else if ge .MemoryUsage 75.0}}bg-yellow{{else}}bg-green{{end}}" style="width: {{.MemoryUsage}}%;"></div>
        </div>
        <div class="card-subtext">{{formatBytes .Data.Memory.UsedBytes}} used | {{formatBytes .Data.Memory.AvailableBytes}} free</div>
        {{if .MemSparkline}}{{.MemSparkline}}{{end}}
      </div>

      <!-- Disk -->
      <div class="card">
        <div class="card-header">
          <span class="card-title">Disk Usage</span>
          <span>💾 {{formatBytes .Data.Disk.TotalBytes}}</span>
        </div>
        <div class="card-value">{{formatFloat .DiskUsage}}%</div>
        <div class="progress-bar-container">
          <div class="progress-bar {{if ge .DiskUsage 90.0}}bg-red{{else if ge .DiskUsage 80.0}}bg-yellow{{else}}bg-blue{{end}}" style="width: {{.DiskUsage}}%;"></div>
        </div>
        <div class="card-subtext">{{formatBytes .Data.Disk.UsedBytes}} used | {{formatBytes .Data.Disk.FreeBytes}} free</div>
      </div>

      <!-- Network -->
      <div class="card">
        <div class="card-header">
          <span class="card-title">Network I/O</span>
          <span>🌐 Throughput</span>
        </div>
        <div class="card-value">{{formatRate .Data.Network.TotalRxRate}}/s</div>
        <div class="card-subtext">Rx Rate: {{formatRate .Data.Network.TotalRxRate}}/s | Tx Rate: {{formatRate .Data.Network.TotalTxRate}}/s</div>
        <div class="card-subtext">Total Rx: {{formatBytes .Data.Network.TotalBytesRecv}} | Total Tx: {{formatBytes .Data.Network.TotalBytesSent}}</div>
        {{if .NetRxSparkline}}{{.NetRxSparkline}}{{end}}
      </div>
    </div>

    <!-- Automated Diagnostics Section -->
    {{if .Data.Diagnostics}}
    <div class="section-card">
      <div class="section-title">
        <span>🩺 Automated Diagnostics Results</span>
        <span class="card-subtext">{{.Data.Diagnostics.PassedChecks}} Passed, {{.Data.Diagnostics.WarningChecks}} Warnings, {{.Data.Diagnostics.CriticalChecks}} Critical</span>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Category</th>
              <th>Rule</th>
              <th>Description</th>
              <th>Remediation / Action</th>
            </tr>
          </thead>
          <tbody>
            {{range .Data.Diagnostics.Results}}
            <tr>
              <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
              <td><strong>{{.Category}}</strong></td>
              <td>{{.Name}}</td>
              <td>{{.Description}}</td>
              <td>{{if .Recommendation}}<span style="color: var(--accent-blue);">{{.Recommendation}}</span>{{else}}<span style="color: var(--text-muted);">-</span>{{end}}</td>
            </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
    {{end}}

    <!-- Statistical Anomalies Section -->
    {{if and .Data.Anomalies (gt (len .Data.Anomalies.Scores) 0)}}
    <div class="section-card">
      <div class="section-title">
        <span>📈 Statistical Anomaly Baseline Evaluation</span>
        <span class="card-subtext">{{.Data.Anomalies.AnomaliesCount}} Anomalies Found</span>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Metric</th>
              <th>Current Value</th>
              <th>Baseline Mean ± StdDev</th>
              <th>Z-Score</th>
              <th>Deviation</th>
              <th>Evaluation</th>
            </tr>
          </thead>
          <tbody>
            {{range .Data.Anomalies.Scores}}
            {{if .IsAnomaly}}
            <tr>
              <td><strong>{{.MetricName}}</strong></td>
              <td>{{formatFloat .CurrentValue}}</td>
              <td>{{formatFloat .Mean}} ± {{formatFloat .StdDev}}</td>
              <td><span class="badge {{severityClass .Severity}}">{{formatFloat .ZScore}}</span></td>
              <td>{{formatFloat .DeviationPct}}%</td>
              <td>{{.Explanation}}</td>
            </tr>
            {{end}}
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
    {{end}}

    <!-- Active Alerts Section -->
    {{if gt (len .Data.ActiveAlerts) 0}}
    <div class="section-card">
      <div class="section-title">
        <span>🚨 Active System Alerts</span>
        <span class="badge badge-critical">{{len .Data.ActiveAlerts}} Active</span>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              <th>Rule</th>
              <th>Metric</th>
              <th>Triggered Value</th>
              <th>Threshold</th>
              <th>Message</th>
              <th>Fired At</th>
            </tr>
          </thead>
          <tbody>
            {{range .Data.ActiveAlerts}}
            <tr>
              <td><span class="badge {{severityClass .Severity}}">{{.Severity}}</span></td>
              <td><strong>{{.RuleName}}</strong></td>
              <td><code>{{.MetricName}}</code></td>
              <td>{{formatFloat .ActualValue}}</td>
              <td>{{formatFloat .Threshold}}</td>
              <td>{{.Message}}</td>
              <td>{{.FiredAt.Format "15:04:05 MST"}}</td>
            </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
    {{end}}

    <!-- Top Processes Section -->
    {{if gt (len .Data.TopProcesses) 0}}
    <div class="section-card">
      <div class="section-title">
        <span>⚙️ Top Resource-Consuming Processes</span>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>PID</th>
              <th>Name</th>
              <th>User</th>
              <th>CPU %</th>
              <th>Mem %</th>
              <th>RSS Memory</th>
              <th>Threads</th>
              <th>Command</th>
            </tr>
          </thead>
          <tbody>
            {{range .Data.TopProcesses}}
            <tr>
              <td><code>{{.PID}}</code></td>
              <td><strong>{{.Name}}</strong></td>
              <td>{{.Username}}</td>
              <td><strong>{{formatFloat .CPUPercent}}%</strong></td>
              <td>{{formatFloat .MemoryPercent}}%</td>
              <td>{{formatBytes .MemoryRSS}}</td>
              <td>{{.NumThreads}}</td>
              <td class="code-tag">{{truncate .CommandLine 60}}</td>
            </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
    {{end}}

    <div class="footer">
      Generated automatically by <strong>Watchdog</strong> (Enterprise System Monitor & Diagnostics CLI)
    </div>
  </div>

  <script>
    function toggleTheme() {
      const current = document.documentElement.getAttribute('data-theme');
      const target = current === 'light' ? 'dark' : 'light';
      document.documentElement.setAttribute('data-theme', target);
      localStorage.setItem('watchdog_theme', target);
    }
    const savedTheme = localStorage.getItem('watchdog_theme');
    if (savedTheme) {
      document.documentElement.setAttribute('data-theme', savedTheme);
    }
  </script>
</body>
</html>
`
