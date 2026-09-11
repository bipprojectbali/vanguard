package panel

import (
	"strings"
	"testing"
)

// tickets_toast_test.go — BL-156e: feedback err/ok modul Tickets/Cases + KB
// Articles + SLA Policies harus toast mengambang (ui.Toast:
// fixed+pointer-events:none+toast-flash), bukan lagi ui.Alert inline statis.
// assertToast: lihat toast_assert_test.go (helper bersama paket ini).

func TestTicketsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	TicketsList(TicketsListView{Err: "Status tidak valid."}).Render(&errOut)
	TicketsList(TicketsListView{Msg: "Status tiket diperbarui."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Status tidak valid.")
	assertToast(t, okOut.String(), "ok", "Status tiket diperbarui.")
}

func TestTicketForm_ToastNotAlert(t *testing.T) {
	var out strings.Builder
	TicketForm(TicketFormView{Err: "Subjek dan desa wajib diisi."}).Render(&out)
	assertToast(t, out.String(), "err", "Subjek dan desa wajib diisi.")
}

func TestKBArticleList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	KBArticleList(KBArticleListView{Err: "Judul artikel wajib diisi."}).Render(&errOut)
	KBArticleList(KBArticleListView{Msg: "Artikel diterbitkan."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Judul artikel wajib diisi.")
	assertToast(t, okOut.String(), "ok", "Artikel diterbitkan.")
}

func TestKBArticleForm_ToastNotAlert(t *testing.T) {
	var out strings.Builder
	KBArticleForm(KBArticleFormView{Err: "Kategori harus salah satu: Panduan Awal, Pembayaran, Kependudukan, Teknis, atau Umum."}).Render(&out)
	assertToast(t, out.String(), "err", "Kategori harus salah satu: Panduan Awal, Pembayaran, Kependudukan, Teknis, atau Umum.")
}

func TestSLAPoliciesList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	SLAPolicyList(SLAPolicyListView{Err: "Nama kebijakan wajib diisi."}).Render(&errOut)
	SLAPolicyList(SLAPolicyListView{Msg: "Kebijakan SLA diaktifkan kembali."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Nama kebijakan wajib diisi.")
	assertToast(t, okOut.String(), "ok", "Kebijakan SLA diaktifkan kembali.")
}

func TestSLAPolicyForm_ToastNotAlert(t *testing.T) {
	var out strings.Builder
	SLAPolicyForm(SLAPolicyFormView{Err: "Target respon/selesai harus berupa menit (angka bulat, tidak negatif)."}).Render(&out)
	assertToast(t, out.String(), "err", "Target respon/selesai harus berupa menit (angka bulat, tidak negatif).")
}
