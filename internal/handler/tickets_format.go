package handler

import (
	"fmt"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_format.go — helper format & pemetaan view tiket: nomor tiket, label SLA,
// durasi, dan ticketRowView/ticketKPIView (db row → panel view). Dipisah dari
// tickets_helpers.go (enum sah + tipe ticketForm + parseTicketForm) agar keduanya
// di bawah ambang tipe Route/Handler (150). Izin/pesan/forbidden ada di
// tickets_view.go. Satu paket; enum tetap SATU sumber.
// ticketNumber memformat nomor tiket yang ditampilkan di UI.
// Format: "#TK-{id}". Disimpan sbg ID di DB, diformat di layer aplikasi.
func ticketNumber(id int64) string {
	return "#TK-" + strconv.FormatInt(id, 10)
}

// formatSLALabel memformat label SLA untuk kolom tabel berdasarkan deadline dan
// status tiket. Kasus khusus:
//   - deadline nil     → "—" (tak ada SLA policy)
//   - status=selesai   → lihat apakah resolved sebelum deadline ("Terpenuhi" / "Terlanggar")
//   - deadline < now() → "Terlanggar" (aktif, sudah lewat)
//   - deadline > now() → waktu tersisa (mis. "1j 20m lagi")
func formatSLALabel(deadline pgtype.Timestamptz, resolvedAt pgtype.Timestamptz, status string) string {
	if !deadline.Valid {
		return "—"
	}
	dl := deadline.Time
	if status == "selesai" {
		if resolvedAt.Valid && resolvedAt.Time.Before(dl) {
			return "Terpenuhi"
		}
		return "Terlanggar"
	}
	now := time.Now()
	if dl.Before(now) {
		return "Terlanggar"
	}
	remaining := dl.Sub(now)
	return formatDuration(remaining) + " lagi"
}

// formatDuration memformat durasi menjadi label singkat (mis. "1j 20m", "45m").
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dj %dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dj", h)
	}
	return fmt.Sprintf("%dm", m)
}

// ticketRowView memetakan satu baris ListTickets → TicketRow view.
func ticketRowView(r db.ListTicketsRow) panel.TicketRow {
	assignedTo := ""
	if r.AssignedToName != nil {
		assignedTo = *r.AssignedToName
	}
	return panel.TicketRow{
		ID:          r.ID,
		Number:      ticketNumber(r.ID),
		AccountName: r.AccountName,
		Subject:     r.Subject,
		Priority:    r.Priority,
		Status:      r.Status,
		SLALabel:    formatSLALabel(r.SlaDeadlineAt, r.ResolvedAt, r.Status),
		AssignedTo:  assignedTo,
	}
}

// ticketKPIView memetakan CountTicketKPIs → TicketKPIs view.
func ticketKPIView(k db.CountTicketKPIsRow) panel.TicketKPIs {
	return panel.TicketKPIs{
		Open:          int(k.OpenCount),
		Unassigned:    int(k.UnassignedCount),
		AtRisk:        int(k.AtRiskCount),
		Breached:      int(k.BreachedCount),
		ResolvedToday: int(k.ResolvedToday),
	}
}
