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

// assertToast didefinisikan LOKAL (bukan diimpor): tiap branch rollout lepas
// dari main sendiri-sendiri, jadi tak ada helper bersama antar branch.
func assertToast(t *testing.T, out, wantText, wantAlertClass string) {
	t.Helper()
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", wantAlertClass, wantText} {
		if !strings.Contains(out, want) {
			t.Errorf("toast kurang %q:\n%s", want, out)
		}
	}
}

func TestCSImplTasksList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSImplTasksList(CSImplTasksListView{Err: "Desa tidak ditemukan atau tidak dalam cakupan Anda."}).Render(&errOut)
	CSImplTasksList(CSImplTasksListView{Msg: "Task berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "Desa tidak ditemukan atau tidak dalam cakupan Anda.", "alert-error")
	assertToast(t, okOut.String(), "Task berhasil dibuat.", "alert-success")
}

func TestCSImplTaskForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSImplTaskForm(CSImplTaskFormView{Err: "Desa dan nama task wajib diisi."}).Render(&errOut)

	assertToast(t, errOut.String(), "Desa dan nama task wajib diisi.", "alert-error")
}

func TestCSRenewalsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSRenewalsList(CSRenewalsListView{Err: "Stage renewal tidak valid."}).Render(&errOut)
	CSRenewalsList(CSRenewalsListView{Msg: "Aksi renewal diperbarui."}).Render(&okOut)

	assertToast(t, errOut.String(), "Stage renewal tidak valid.", "alert-error")
	assertToast(t, okOut.String(), "Aksi renewal diperbarui.", "alert-success")
}

func TestCSRenewalForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSRenewalForm(CSRenewalFormView{Err: "Anggota tidak ditemukan."}).Render(&errOut)

	assertToast(t, errOut.String(), "Anggota tidak ditemukan.", "alert-error")
}

func TestCSTrainingsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSTrainingsList(CSTrainingsListView{Err: "Format tanggal training tidak valid."}).Render(&errOut)
	CSTrainingsList(CSTrainingsListView{Msg: "Jadwal training berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "Format tanggal training tidak valid.", "alert-error")
	assertToast(t, okOut.String(), "Jadwal training berhasil dibuat.", "alert-success")
}

func TestCSTrainingForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CSTrainingForm(CSTrainingFormView{Err: "Status tidak valid."}).Render(&errOut)

	assertToast(t, errOut.String(), "Status tidak valid.", "alert-error")
}

func TestEngagementsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	EngagementsList(EngagementsListView{Err: "Format tanggal jadwal tidak valid."}).Render(&errOut)
	EngagementsList(EngagementsListView{Msg: "Engagement berhasil dibuat."}).Render(&okOut)

	assertToast(t, errOut.String(), "Format tanggal jadwal tidak valid.", "alert-error")
	assertToast(t, okOut.String(), "Engagement berhasil dibuat.", "alert-success")
}

func TestEngagementForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	EngagementForm(EngagementFormView{Err: "Tipe engagement tidak valid."}).Render(&errOut)

	assertToast(t, errOut.String(), "Tipe engagement tidak valid.", "alert-error")
}

func TestCustomerSuccessDetail_ToastNotAlert(t *testing.T) {
	var okOut strings.Builder
	CustomerSuccessDetail(CustomerSuccessDetailView{
		Base: "/w/acme", ID: 1, AccountName: "Desa Contoh", Exists: true,
		Msg: "Perubahan Customer Success disimpan.",
	}).Render(&okOut)

	assertToast(t, okOut.String(), "Perubahan Customer Success disimpan.", "alert-success")
}

func TestCustomerSuccessForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	CustomerSuccessForm(CustomerSuccessFormView{Err: "Skor harus bilangan bulat 0–100."}).Render(&errOut)

	assertToast(t, errOut.String(), "Skor harus bilangan bulat 0–100.", "alert-error")
}

func TestCSJourneyList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	CSJourneyList(CSJourneyListView{Err: "Galat contoh."}).Render(&errOut)
	CSJourneyList(CSJourneyListView{Msg: "Sukses contoh."}).Render(&okOut)

	assertToast(t, errOut.String(), "Galat contoh.", "alert-error")
	assertToast(t, okOut.String(), "Sukses contoh.", "alert-success")
}
