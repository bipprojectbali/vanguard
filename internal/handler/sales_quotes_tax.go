package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_tax.go — AKSI pajak Quote (BL-14). Pajak DIPINDAH dari header ke
// BUILDER: di sini subtotal sudah hidup, jadi mode 'percent' (mis. PPN 11%) bisa
// dihitung dari Σ item dan MENGIKUTI perubahan item otomatis (recomputeTotals dipanggil
// juga tiap item berubah). Mode 'amount' = nilai rupiah tetap (mis. materai). Gerbang
// SAMA dgn mutasi quote lain (requireDealWrite + BL-13 requireQuotableStage). Navigasi
// native POST → 303 (gotcha #16), BUKAN sse.Redirect (diblokir CSP).

const (
	taxModePercent = "percent" // tarif persen dari subtotal (tax_rate dipakai)
	taxModeAmount  = "amount"  // nilai rupiah tetap (tax_rate NULL) — juga default DB
	defaultTaxRate = "11"      // prefill tarif saat kontrol pajak belum dikonfigurasi (PPN)
	maxTaxRate     = 100       // batas atas tarif persen (masuk akal + muat NUMERIC(5,2))
)

// trimDecZeros merapikan desimal kanonik Postgres untuk prefill input: "11.00"→"11",
// "10000.50"→"10000.5", ""→"". Tanpa titik = apa adanya. Prefill lebih bersih; nilai
// tetap sah diparse ulang saat submit.
func trimDecZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// parseTaxForm membaca tax_mode + nilai yang relevan dengan mode terpilih. Field mode
// lain SENGAJA diabaikan (keduanya selalu terkirim oleh toggle Datastar; backend hanya
// membaca yang cocok — pola sama dealStageControl BL-12). (mode, rate, amount, "") bila
// sah; ("", zero, zero, kode) bila tolak (dipetakan wsErrMsgCRM).
//
//   - percent → tax_rate WAJIB angka 0..100 (0 = tanpa pajak via persen); amount NULL.
//   - amount  → tax_amount opsional (kosong = NULL = tanpa pajak); terisi wajib ≥ 0;
//     rate NULL.
func parseTaxForm(fv func(string) string) (mode string, rate, amount pgtype.Numeric, errCode string) {
	switch fv("tax_mode") {
	case taxModePercent:
		r, code := optNumeric(fv("tax_rate"), "tax_rate")
		if code != "" {
			return "", pgtype.Numeric{}, pgtype.Numeric{}, code
		}
		if !r.Valid || !numericBetween(r, 0, maxTaxRate) {
			return "", pgtype.Numeric{}, pgtype.Numeric{}, "tax_rate"
		}
		return taxModePercent, r, pgtype.Numeric{Valid: false}, ""
	case taxModeAmount:
		// BL-147: Nominal = rupiah BULAT → buang pemisah ribuan ("5.000.000" →
		// "5000000") sebelum parse, selaras field uang lain. tax_rate TIDAK
		// dibersihkan (di sana titik = pemisah desimal persen).
		a, code := optNumeric(cleanThousands(fv("tax_amount")), "tax")
		if code != "" {
			return "", pgtype.Numeric{}, pgtype.Numeric{}, code
		}
		if a.Valid && !numericNonNegative(a) {
			return "", pgtype.Numeric{}, pgtype.Numeric{}, "tax"
		}
		return taxModeAmount, pgtype.Numeric{Valid: false}, a, ""
	default:
		return "", pgtype.Numeric{}, pgtype.Numeric{}, "tax_mode"
	}
}

// quoteTaxView menurunkan data pajak untuk builder (BL-14): mode aktif, prefill input
// tiap mode (desimal dirapikan), & label baris ringkasan. Default Persentase 11% saat
// quote BELUM dikonfigurasi pajaknya (mode 'amount' warisan default DB TANPA nilai) →
// kontrol langsung menawarkan jalur PPN yang lazim; baris lama hasil migrasi (mode
// 'amount' + nilai tersimpan) tetap Nominal agar rupiah manualnya utuh.
func quoteTaxView(q db.Quote) (mode, rateInput, amountInput, label string) {
	rateInput = defaultTaxRate
	label = "Pajak"
	if q.TaxMode == taxModePercent {
		if r := trimDecZeros(numericStr(q.TaxRate)); r != "" {
			rateInput = r
		}
		return taxModePercent, rateInput, "", "Pajak (" + rateInput + "%)"
	}
	if q.TaxAmount.Valid { // mode 'amount' dgn nilai → Nominal (materai / baris lama)
		return taxModeAmount, rateInput, trimDecZeros(numericStr(q.TaxAmount)), label
	}
	return taxModePercent, rateInput, "", label // belum dikonfigurasi → default persen
}

// QuoteTax — POST /w/{workspace}/deals/{id}/quotes/{quoteID}/tax. Set mode & nilai pajak
// lalu rekalkulasi total (recomputeTotals). Gerbang identik mutasi quote: requireDealWrite
// (sumbu crm:deals) + BL-13 requireQuotableStage (terminal/dini = beku).
func (h *Handler) QuoteTax(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	dealID, quoteID, ok := h.parseQuoteRef(w, r)
	if !ok {
		return
	}
	_, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	// BL-13: ubah pajak hanya di jendela quoting (arsip/dini read-only).
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
		return
	}
	mode, rate, amount, errCode := parseTaxForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, quoteSub(dealID, quoteID), errCode)
		return
	}

	uid := session.UserID(ctx)
	if err := h.recomputeTotals(ctx, quoteID, mode, rate, amount, uid); err != nil {
		h.Log.Error("quotes: tax recompute", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.tax", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10), "mode": mode,
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "tax_saved")
}
