package panel

import (
	"strings"
	"testing"
)

// sales_toast_test.go — BL-156c: feedback err/ok/msg pada seluruh halaman Sales
// (activities/deals/leads/quotes) harus toast mengambang (ui.Toast:
// fixed+pointer-events:none+toast-flash), bukan lagi ui.Alert inline statis.
// Juga mengunci dua celah yang ditemukan (ActivityDetailView & LeadDetailView
// sebelumnya tak punya slot Err/Msg lengkap) agar tak regresi.

func assertToast(t *testing.T, out, kind, msg string) {
	t.Helper()
	alertClass := "alert-error"
	if kind == "ok" {
		alertClass = "alert-success"
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", alertClass, msg} {
		if !strings.Contains(out, want) {
			t.Errorf("toast %s kurang %q:\n%s", kind, want, out)
		}
	}
}

func TestActivitiesList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	ActivitiesList(ActivitiesListView{Err: "Gagal memuat aktivitas."}).Render(&errOut)
	ActivitiesList(ActivitiesListView{Msg: "Aktivitas dicatat."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat aktivitas.")
	assertToast(t, okOut.String(), "ok", "Aktivitas dicatat.")
}

func TestActivityForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	ActivityForm(ActivityFormView{Err: "Subjek wajib diisi."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Subjek wajib diisi.")
}

func TestActivityDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	ActivityDetail(ActivityDetailView{Err: "Status tidak valid."}).Render(&errOut)
	ActivityDetail(ActivityDetailView{Msg: "Status aktivitas diperbarui."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Status tidak valid.")
	assertToast(t, okOut.String(), "ok", "Status aktivitas diperbarui.")
}

func TestDealPipeline_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	DealPipeline(DealPipelineView{Err: "Gagal memuat deal."}).Render(&errOut)
	DealPipeline(DealPipelineView{Msg: "Deal ditambahkan."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat deal.")
	assertToast(t, okOut.String(), "ok", "Deal ditambahkan.")
}

func TestDealDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	DealDetail(DealDetailView{Err: "Tahap tidak dapat diubah."}).Render(&errOut)
	DealDetail(DealDetailView{Msg: "Deal dibuat dari konversi lead."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Tahap tidak dapat diubah.")
	assertToast(t, okOut.String(), "ok", "Deal dibuat dari konversi lead.")
}

func TestDealForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	DealForm(DealFormView{Err: "Nama deal wajib diisi."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Nama deal wajib diisi.")
}

func TestLeadsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	LeadsList(LeadsListView{Err: "Gagal memuat lead."}).Render(&errOut)
	LeadsList(LeadsListView{Msg: "Lead ditambahkan."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat lead.")
	assertToast(t, okOut.String(), "ok", "Lead ditambahkan.")
}

func TestLeadForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	LeadForm(LeadFormView{Err: "Nama lead wajib diisi."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Nama lead wajib diisi.")
}

// TestLeadDetail_ToastNotAlert mengunci celah BL-156c: sebelumnya LeadDetailView
// punya Err tapi TAK punya Msg — sukses ubah status ditelan senyap.
func TestLeadDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	LeadDetail(LeadDetailView{Err: "Status tidak valid."}).Render(&errOut)
	LeadDetail(LeadDetailView{Msg: "Perubahan lead disimpan."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Status tidak valid.")
	assertToast(t, okOut.String(), "ok", "Perubahan lead disimpan.")
}

func TestQuotesList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	QuotesList(QuotesListView{Err: "Gagal memuat quote."}).Render(&errOut)
	QuotesList(QuotesListView{Msg: "Quote dibuat."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat quote.")
	assertToast(t, okOut.String(), "ok", "Quote dibuat.")
}

func TestQuoteDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	QuoteDetail(QuoteDetailView{Err: "Item tidak ditemukan."}).Render(&errOut)
	QuoteDetail(QuoteDetailView{OK: "Item ditambahkan ke quote."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Item tidak ditemukan.")
	assertToast(t, okOut.String(), "ok", "Item ditambahkan ke quote.")
}

func TestQuoteForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	QuoteForm(QuoteFormView{Err: "Tanggal kedaluwarsa tidak valid."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Tanggal kedaluwarsa tidak valid.")
}

func TestQuotesIndex_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	QuotesIndex(QuotesIndexView{Err: "Gagal memuat quote."}).Render(&errOut)
	QuotesIndex(QuotesIndexView{Msg: "Quote dibuat."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat quote.")
	assertToast(t, okOut.String(), "ok", "Quote dibuat.")
}

func TestLeadConvert_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	LeadConvert(LeadConvertView{Err: "Desa sudah dipakai lead lain."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Desa sudah dipakai lead lain.")
}

// TestLeadConvert_DupAccountLink mengunci jalur convertErrAlert dgn DupAccountID
// != 0 (BL-67): pesan + tautan ke akun eksisting, tetap toast (bukan alert).
func TestLeadConvert_DupAccountLink(t *testing.T) {
	var out strings.Builder
	LeadConvert(LeadConvertView{
		Base:            "/w/acme",
		Err:             "Desa ini sudah punya akun.",
		DupAccountID:    42,
		DupAccountLabel: "Desa Sukamaju",
	}).Render(&out)
	assertToast(t, out.String(), "err", "Desa ini sudah punya akun.")
	for _, want := range []string{"/w/acme/accounts/42", "Desa Sukamaju"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("toast dup-account kurang %q:\n%s", want, out.String())
		}
	}
}
