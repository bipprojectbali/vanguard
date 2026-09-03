package handler

import (
	"strconv"
)

// tickets_helpers.go — form struct, enum, dan mapper untuk Tickets / Cases
// (Modul 6 slice B2). Murni Go (tanpa HTTP/DB call) agar bisa diuji langsung.
// Meniru customer_success_helpers.go.

// ticketPriorityValues = nilai enum sahih, cerminan CHECK priority constraint
// migrasi 00020. Compile-time sync check di init().
var ticketPriorityValues = []string{"rendah", "sedang", "tinggi"}

// ticketStatusValues = nilai enum sahih, cerminan CHECK status constraint
// migrasi 00036 (BL-38): 4 fase inti. Reopen bukan status — tombol yang menulis
// 'diproses' (Selesai→Diproses).
var ticketStatusValues = []string{"baru", "diproses", "menunggu", "selesai"}

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
