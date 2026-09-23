package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderNetworkView() string {
	var sb strings.Builder

	// Top Network Overview
	if m.currentSnapshot != nil && m.currentSnapshot.Network != nil {
		net := m.currentSnapshot.Network
		var ifaceSb strings.Builder
		ifaceSb.WriteString(SubTitleStyle.Render("🌐 Active Network Interfaces & Throughput") + "\n")
		ifaceSb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-16s %-18s %-16s %-12s %-12s", "NAME", "IP ADDRESSES", "MAC", "RX RATE", "TX RATE")) + "\n")
		ifaceSb.WriteString("  " + strings.Repeat("-", m.width-12) + "\n")

		for _, iface := range net.Interfaces {
			ips := strings.Join(iface.Addrs, ", ")
			ifaceSb.WriteString(fmt.Sprintf("  %-16s %-18s %-16s %-12s %-12s\n",
				TruncateString(iface.Name, 14),
				TruncateString(ips, 16),
				TruncateString(iface.HardwareAddr, 14),
				FormatRate(iface.RxRate),
				FormatRate(iface.TxRate),
			))
		}
		sb.WriteString(CardStyle.Width(m.width - 4).Render(ifaceSb.String()) + "\n\n")
	}

	// Listening Ports Table
	sb.WriteString(SubTitleStyle.Render(fmt.Sprintf("🔌 Listening Ports & Sockets (%d discovered)", len(m.ports))) + "\n")
	if len(m.ports) == 0 {
		sb.WriteString(CardStyle.Width(m.width - 4).Render(
			MutedStyle.Render("No active listening sockets discovered or scanning in progress..."),
		))
	} else {
		var portsSb strings.Builder
		header := fmt.Sprintf("  %-8s %-8s %-20s %-8s %-18s %s", "PORT", "PROTO", "INTERFACE / IP", "PID", "PROCESS", "SERVICE")
		portsSb.WriteString(TableHeaderStyle.Width(m.width - 8).Render(header) + "\n")

		maxRows := m.height - 20
		if maxRows < 5 {
			maxRows = 5
		}
		end := m.portScrollOff + maxRows
		if end > len(m.ports) {
			end = len(m.ports)
		}

		for i := m.portScrollOff; i < end; i++ {
			p := m.ports[i]
			row := fmt.Sprintf("  %-8d %-8s %-20s %-8d %-18s %s",
				p.Port,
				strings.ToUpper(p.Protocol),
				TruncateString(p.Interface, 18),
				p.PID,
				TruncateString(p.ProcessName, 16),
				TruncateString(p.Service, 20),
			)

			if i == m.selectedPortIdx {
				portsSb.WriteString(TableRowSelectedStyle.Width(m.width - 8).Render(row) + "\n")
			} else {
				portsSb.WriteString(row + "\n")
			}
		}

		sb.WriteString(CardStyle.Width(m.width - 4).Render(portsSb.String()))
	}

	return sb.String()
}
