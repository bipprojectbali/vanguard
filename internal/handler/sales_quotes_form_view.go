package handler

import (
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_form_view.go — pemetaan BALIK quote tersimpan → prefill form
// header, dan penanda kedaluwarsa computed-on-read, dipisah dari
// sales_quotes_form.go (parsing form) untuk file health.

// quoteFormFields memetakan quote termuat → prefill form header (semua string).
func quoteFormFields(q db.Quote) panel.QuoteFormFields {
	preparedBy := ""
	if q.PreparedBy != nil {
		preparedBy = strconv.FormatInt(*q.PreparedBy, 10)
	}
	return panel.QuoteFormFields{
		QuoteName:        deref(q.QuoteName),
		ExpirationDate:   dateStr(q.ExpirationDate),
		PaymentTerms:     deref(q.PaymentTerms),
		NotesTerms:       deref(q.NotesTerms),
		PreparedBy:       preparedBy,
		SubscriptionTerm: deref(q.SubscriptionTerm),
	}
}

// quoteExpired = penanda kedaluwarsa computed-on-read (BL-17), di-precompute di
// handler (view murni-data). Draft SELALU dikecualikan (masih WIP, belum
// ditawarkan). True hanya bila status non-Draft, expiration_date terisi, DAN
// tanggalnya SEBELUM hari ini (banding date-only). Karena parseQuoteForm memblok
// simpan tanggal lampau, sebuah quote hanya bisa menjadi kedaluwarsa akibat waktu
// berjalan setelah disimpan dengan tanggal depan.
func quoteExpired(status string, exp pgtype.Date, today time.Time) bool {
	if status == "Draft" || !exp.Valid {
		return false
	}
	return dateBefore(exp.Time, today)
}
