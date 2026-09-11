package panel

import (
	"strings"
	"testing"
)

// subscriptions_toast_test.go — BL-156d: feedback err/ok pada halaman
// Subscriptions/Plans/Playbooks harus toast mengambang (ui.Toast:
// fixed+pointer-events:none+toast-flash), bukan lagi ui.Alert inline statis.

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

func TestSubList_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	SubList(SubListView{Err: "Gagal memuat langganan."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat langganan.")
}

func TestSubDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	SubDetail(SubDetailView{Err: "Langganan belum aktif."}).Render(&errOut)
	SubDetail(SubDetailView{Msg: "Langganan diperpanjang — periode baru aktif."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Langganan belum aktif.")
	assertToast(t, okOut.String(), "ok", "Langganan diperpanjang — periode baru aktif.")
}

func TestChurnList_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	ChurnList(ChurnView{Err: "Langganan tidak aktif."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Langganan tidak aktif.")
}

func TestRenewalsList_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	RenewalsList(RenewalsView{Err: "Gagal memuat renewal."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat renewal.")
}

func TestPlanList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	PlanList(PlanListView{Err: "Kode plan sudah dipakai."}).Render(&errOut)
	PlanList(PlanListView{Msg: "Plan ditambahkan ke katalog."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Kode plan sudah dipakai.")
	assertToast(t, okOut.String(), "ok", "Plan ditambahkan ke katalog.")
}

func TestPlanForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	PlanForm(PlanFormView{Err: "Nama dan kode plan wajib diisi."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Nama dan kode plan wajib diisi.")
}

func TestPlaybookList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	PlaybookList(PlaybookListView{Err: "Gagal memuat playbook."}).Render(&errOut)
	PlaybookList(PlaybookListView{Msg: "Playbook ditambahkan ke katalog."}).Render(&okOut)
	assertToast(t, errOut.String(), "err", "Gagal memuat playbook.")
	assertToast(t, okOut.String(), "ok", "Playbook ditambahkan ke katalog.")
}

func TestPlaybookForm_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	PlaybookForm(PlaybookFormView{Err: "Nama playbook wajib diisi."}).Render(&errOut)
	assertToast(t, errOut.String(), "err", "Nama playbook wajib diisi.")
}
