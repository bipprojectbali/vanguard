package handler

import (
	"fmt"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_helpers.go — form struct, enum, dan mapper untuk Tickets / Cases
// (Modul 6 slice B2). Murni Go (tanpa HTTP/DB call) agar bisa diuji langsung.
// Meniru customer_success_helpers.go.

// ticketPriorityValues = nilai enum sahih, cerminan CHECK priority constraint
// migrasi 00020. Compile-time sync check di init().
var ticketPriorityValues = []string{"rendah", "sedang", "tinggi"}

// ticketStatusValues = nilai enum sahih, cerminan CHECK status constraint
// migrasi 00020.
var ticketStatusValues = []string{"baru", "ditugaskan", "eskalasi", "selesai"}

func init() {
	// Guard compile-time: panjang harus sesuai (jika enum migrasi bertambah,
	// tambah di sini — test regresi akan gagal sebelum ini).
	if len(ticketPriorityValues) != 3 {
		panic("ticketPriorityValues: jumlah tidak sinkron dengan enum migrasi")
	}
	if len(ticketStatusValues) != 4 {
		panic("ticketStatusValues: jumlah tidak sinkron dengan enum migrasi")
	}
}

// ticketForm = data terurai dari form POST tiket baru.
type ticketForm struct {
	AccountID   int64
	Subject     string
	Description *string
	Priority    string
	AssignedTo  *int64
	SlaPolicyID *int64
}

// parseTicketForm terurai dan memvalidasi form tiket baru. Mengembalikan
// (form, "") jika OK; ("", errCode) jika validasi gagal.
func parseTicketForm(fv func(string) string) (ticketForm, string) {
	accountIDStr := fv("account_id")
	if accountIDStr == "" {
		return ticketForm{}, "required"
	}
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		return ticketForm{}, "account"
	}

	subject := fv("subject")
	if subject == "" {
		return ticketForm{}, "required"
	}

	priority := fv("priority")
	if !isValidEnum(priority, ticketPriorityValues) {
		return ticketForm{}, "priority"
	}

	f := ticketForm{
		AccountID: accountID,
		Subject:   subject,
		Priority:  priority,
	}

	if desc := fv("description"); desc != "" {
		f.Description = &desc
	}

	if s := fv("assigned_to"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.AssignedTo = &id
		}
	}

	if s := fv("sla_policy_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.SlaPolicyID = &id
		}
	}

	return f, ""
}

// isValidEnum melaporkan apakah val ada di vals.
func isValidEnum(val string, vals []string) bool {
	for _, v := range vals {
		if v == val {
			return true
		}
	}
	return false
}

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
