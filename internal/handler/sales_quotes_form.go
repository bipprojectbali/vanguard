package handler

import (
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_form.go — parse & validasi form HEADER Quote, dan pemetaan
// baliknya (quote tersimpan → field form prefill saat sunting). Dipisah dari
// aksi (sales_quotes.go/sales_quotes_page.go) & form ITEM (sales_quote_items_form.go)
// agar aturan "kosong = NULL / terisi = wajib sah" punya SATU tempat: create &
// edit tak boleh menerima nilai yang berbeda sahnya untuk kolom yang sama.
//
// Status quote di sini = CERMIN CHECK constraint migrasi 00010 (quotes_status_chk):
// penolakan terjadi di sini SEBELUM DB; CHECK jaring terakhir. Pajak TIDAK lagi di
// header (BL-14) — dipindah ke builder item (QuoteTax) tempat subtotal sudah hidup;
// header hanya identitas quote (nama, tanggal, termin, penyusun).

const (
	maxQuoteNameLen  = 200  // quote_name (nullable)
	maxQuoteTermsLen = 2000 // payment_terms / notes_terms — teks bebas, guard luapan
)

// quoteInitialStatus = status setiap quote baru. Transisi berikutnya = aksi manual
// tersendiri (UpdateQuoteStatus); approval flow ditunda (keputusan scope).
const quoteInitialStatus = "Draft"

// validQuoteStatuses = himpunan status legal (cermin quotes_status_chk 00010).
// Approval flow ditunda → semua status boleh diset manual dari kontrol status.
var validQuoteStatuses = map[string]struct{}{
	"Draft": {}, "Sent": {}, "Under Review": {},
	"Accepted": {}, "Rejected": {}, "Expired": {},
}

// quoteStatusOptions = status untuk dropdown, BERURUT (map validasi tak berurut).
// Harus himpunan yang sama dengan validQuoteStatuses & CHECK 00010.
var quoteStatusOptions = []string{"Draft", "Sent", "Under Review", "Accepted", "Rejected", "Expired"}

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(quoteStatusOptions) != len(validQuoteStatuses) {
		panic("quotes: opsi status tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// quoteForm = nilai form HEADER quote yang SUDAH divalidasi. Semua kolom opsional
// (quote menempel ke deal induk untuk account/deal_id). PreparedBy *int64 (nil = tak
// ditunjuk). Pajak bukan lagi bagian header (BL-14) → dikelola QuoteTax di builder.
type quoteForm struct {
	QuoteName          *string
	ExpirationDate     pgtype.Date
	PaymentTerms       *string
	NotesTerms         *string
	PreparedBy         *int64
	SubscriptionTerm   *string // BL-88: termin langganan milik quote (bukan deal lagi)
	ContractTermMonths *int32  // diturunkan dari SubscriptionTerm (peta termContractMonths)
}

// parseQuoteForm membaca & memvalidasi form header. (form, "") bila sah, atau
// (zero, kode) yang dipetakan wsErrMsg. account_id & deal_id TAK di sini —
// diwarisi dari deal induk oleh handler (alur nest). today = "hari ini" (zona
// aplikasi) dioper masuk agar parser tetap murni & create/edit pakai SATU aturan
// kedaluwarsa (BL-17): expiration_date < today ditolak (termasuk saat Draft).
func parseQuoteForm(fv func(string) string, today time.Time) (quoteForm, string) {
	var f quoteForm

	// quote_name opsional: kosong = NULL; terisi wajib ≤ batas.
	if s := strings.TrimSpace(fv("quote_name")); s != "" {
		if len(s) > maxQuoteNameLen {
			return quoteForm{}, "quote_name"
		}
		f.QuoteName = &s
	}

	// expiration_date opsional (belum ditetapkan = NULL).
	ed, code := optDate(fv("expiration_date"))
	if code != "" {
		return quoteForm{}, code
	}
	// BL-17: tanggal kedaluwarsa terisi tak boleh SEBELUM hari ini (banding date-
	// only; == hari ini boleh, berlaku s/d akhir hari). Penjaga sesungguhnya di
	// backend; input date juga diberi min di klien (jaring, bukan pengaman).
	if ed.Valid && dateBefore(ed.Time, today) {
		return quoteForm{}, "expiration_past"
	}
	f.ExpirationDate = ed

	// payment_terms / notes_terms teks bebas opsional (guard panjang).
	if pt := optTrim(fv("payment_terms")); pt != nil {
		if len(*pt) > maxQuoteTermsLen {
			return quoteForm{}, "terms"
		}
		f.PaymentTerms = pt
	}
	if nt := optTrim(fv("notes_terms")); nt != nil {
		if len(*nt) > maxQuoteTermsLen {
			return quoteForm{}, "terms"
		}
		f.NotesTerms = nt
	}

	// prepared_by opsional: kosong = NULL; terisi wajib id positif. Picker hanya
	// menawarkan anggota workspace; id tak dikenal ternetralkan saat resolusi nama.
	pb, code := optPositiveID(fv("prepared_by"), "prepared_by")
	if code != "" {
		return quoteForm{}, code
	}
	f.PreparedBy = pb

	// BL-88: subscription_term opsional (kosong = NULL; termin belum ditetapkan).
	// Terisi wajib enum (cermin quotes_term_chk 00041 = deals_term_chk). Bulan-kontrak
	// diturunkan dari peta termContractMonths (sumber tunggal, dipakai juga Won→Langganan)
	// → tersimpan di contract_term_months agar Won tak menebak lagi.
	if s := strings.TrimSpace(fv("subscription_term")); s != "" {
		if _, ok := validSubscriptionTerms[s]; !ok {
			return quoteForm{}, "deal_term"
		}
		f.SubscriptionTerm = &s
		if m, ok := termContractMonths[s]; ok {
			months := m
			f.ContractTermMonths = &months
		}
	}

	return f, ""
}

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
