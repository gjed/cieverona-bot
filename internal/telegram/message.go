package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/gjed/cie-verona/internal/booking"
)

const bookingURL = "https://www.comune.verona.it/prenota_appuntamento?service_id=5916"

// BuildMessage formats findings into a Telegram HTML message.
func BuildMessage(findings []booking.Finding, months []string, errs []string) string {
	var sb strings.Builder

	sb.WriteString("<b>🆔 CIE Verona – appuntamenti disponibili</b>\n")
	sb.WriteString(fmt.Sprintf("Mesi: %s\n", strings.Join(months, ", ")))
	sb.WriteString(fmt.Sprintf("Verifica: %s\n\n", time.Now().Format("02/01/2006 15:04")))

	// Group findings by GroupName, preserving order of first appearance.
	grouped := map[string][]booking.Finding{}
	var order []string
	seen := map[string]bool{}
	for _, f := range findings {
		if !seen[f.GroupName] {
			order = append(order, f.GroupName)
			seen[f.GroupName] = true
		}
		grouped[f.GroupName] = append(grouped[f.GroupName], f)
	}

	for _, gName := range order {
		sb.WriteString(fmt.Sprintf("<b>%s</b>\n", escape(gName)))
		for _, f := range grouped[gName] {
			sb.WriteString(fmt.Sprintf(
				"  • %s — %s (%d slot)\n    <i>%s</i>\n",
				escape(f.Date),
				escape(f.CalendarName),
				f.SlotCount,
				escape(f.Location),
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("<a href=\"%s\">Prenota appuntamento</a>", bookingURL))

	if len(errs) > 0 {
		sb.WriteString("\n\n<b>Errori:</b>\n")
		for _, e := range errs {
			sb.WriteString(fmt.Sprintf("  • %s\n", escape(e)))
		}
	}

	return sb.String()
}

// escape escapes characters significant in Telegram HTML mode.
func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
