package panel

import (
	"strings"
	"testing"
)

// customer_success_toast_test.go — BL-156g: feedback err/ok di seluruh modul
// customer-success (Customer Success detail+form, Implementation Tasks,
// Renewal Management, Training Schedule, Engagements, Journey) harus toast
// mengambang (ui.Toast: fixed+pointer-events:none+toast-flash), bukan lagi
// ui.Alert inline statis maupun div mentah. Meniru pola accounts_toast_test.go
// (BL-156a) / roles_members_toast_test.go (BL-156f).

// assertToast: lihat toast_assert_test.go (helper bersama paket ini).

func TestCSImplTasksList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSImplTasksList(CSImplTasksListView{Err: "Desa tidak ditemukan atau tidak dalam cakupan Anda."}).Render(&errOut)
	CSImplTasksList(CSImplTasksListView{Msg: "Task berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Desa tidak ditemukan atau tidak dalam cakupan Anda.")
	assertToast(t, okOut.String(), "ok", "Task berhasil dibuat.")
}

func TestCSImplTaskForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSImplTaskForm(CSImplTaskFormView{Err: "Desa dan nama task wajib diisi."}).Render(&errOut)

	assertToast(t, errOut.String(), "err", "Desa dan nama task wajib diisi.")
}

func TestCSRenewalsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSRenewalsList(CSRenewalsListView{Err: "Stage renewal tidak valid."}).Render(&errOut)
	CSRenewalsList(CSRenewalsListView{Msg: "Aksi renewal diperbarui."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Stage renewal tidak valid.")
	assertToast(t, okOut.String(), "ok", "Aksi renewal diperbarui.")
}

func TestCSRenewalForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSRenewalForm(CSRenewalFormView{Err: "Anggota tidak ditemukan."}).Render(&errOut)

	assertToast(t, errOut.String(), "err", "Anggota tidak ditemukan.")
}

func TestCSTrainingsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSTrainingsList(CSTrainingsListView{Err: "Format tanggal training tidak valid."}).Render(&errOut)
	CSTrainingsList(CSTrainingsListView{Msg: "Jadwal training berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Format tanggal training tidak valid.")
	assertToast(t, okOut.String(), "ok", "Jadwal training berhasil dibuat.")
}

func TestCSTrainingForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSTrainingForm(CSTrainingFormView{Err: "Status tidak valid."}).Render(&errOut)

	assertToast(t, errOut.String(), "err", "Status tidak valid.")
}

func TestEngagementsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	EngagementsList(EngagementsListView{Err: "Format tanggal jadwal tidak valid."}).Render(&errOut)
	EngagementsList(EngagementsListView{Msg: "Engagement berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Format tanggal jadwal tidak valid.")
	assertToast(t, okOut.String(), "ok", "Engagement berhasil dibuat.")
}

func TestEngagementForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	EngagementForm(EngagementFormView{Err: "Tipe engagement tidak valid."}).Render(&errOut)

	assertToast(t, errOut.String(), "err", "Tipe engagement tidak valid.")
}

// TestCustomerSuccessDetail_SavedMsg_ToastNotAlert: pesan sukses SAVE
// (customerSuccessMsg "saved") — beda dari TestCustomerSuccessDetail_ToastNotAlert
// di accounts_toast_test.go (BL-156b) yang menguji jalur AccountAssign
// (accountsMsg "assigned"). Dua sumber ?ok= berbeda, mendarat di halaman sama.
func TestCustomerSuccessDetail_SavedMsg_ToastNotAlert(t *testing.T) {
	var okOut strings.Builder
	CustomerSuccessDetail(CustomerSuccessDetailView{
		Base: "/w/acme", ID: 1, AccountName: "Desa Contoh", Exists: true,
		Msg: "Perubahan Customer Success disimpan.",
	}).Render(&okOut)

	assertToast(t, okOut.String(), "ok", "Perubahan Customer Success disimpan.")
}

func TestCustomerSuccessForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CustomerSuccessForm(CustomerSuccessFormView{Err: "Skor harus bilangan bulat 0–100."}).Render(&errOut)

	assertToast(t, errOut.String(), "err", "Skor harus bilangan bulat 0–100.")
}

func TestCSJourneyList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSJourneyList(CSJourneyListView{Err: "Galat contoh."}).Render(&errOut)
	CSJourneyList(CSJourneyListView{Msg: "Sukses contoh."}).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Galat contoh.")
	assertToast(t, okOut.String(), "ok", "Sukses contoh.")
}
