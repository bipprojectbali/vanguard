package panel

import (
	"strings"
	"testing"
)

// customer_success_view_test.go — regresi BL-108: penugasan CS (CS Utama/
// Cadangan) dipindah dari form edit desa ke halaman detail Customer Success,
// dibuka lewat MODAL (revisi 9 Sep — bukan kartu inline). Gerbang tampil =
// CanAssign (dihitung handler dari crm:accounts write & tak read-only). Diuji di
// level view murni karena tak ada role bawaan yang bisa MEMBACA halaman CS tapi
// TAK berhak tulis account lewat HTTP nyata (support ber-accounts-read ditolak
// F3; sisanya semua ber-accounts-write) — sejajar pola masking per-section yang
// juga diuji di level fungsi.

func renderCSDetail(t *testing.T, v CustomerSuccessDetailView) string {
	t.Helper()
	var sb strings.Builder
	if err := CustomerSuccessDetail(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestCustomerSuccessDetail_AssignCanAssign: CanAssign=true → tombol pembuka +
// MODAL Penugasan CS (form posting ke AssignAction) dirender. Diuji pada
// empty-state (Exists=false) sebab penugasan harus bisa dilakukan sebelum data
// CS pernah diisi. Signal Datastar $assignOpen mengunci pola modal (bukan kartu
// inline) — pengendali buka/tutup dialog.
func TestCustomerSuccessDetail_AssignCanAssign(t *testing.T) {
	v := CustomerSuccessDetailView{
		Base: "/w/test", ID: 7, AccountName: "Desa X", Exists: false,
		CanAssign:    true,
		AssignAction: "/w/test/accounts/7/assign",
		Members:      []AccountMemberOption{{ID: 3, Label: "Budi"}},
	}
	body := renderCSDetail(t, v)
	if !strings.Contains(body, "Penugasan CS") {
		t.Error("BL-108: CanAssign=true harus menampilkan Penugasan CS")
	}
	if !strings.Contains(body, "/w/test/accounts/7/assign") {
		t.Error("BL-108: form Penugasan CS harus posting ke AssignAction")
	}
	if !strings.Contains(body, "assignOpen") {
		t.Error("BL-108: penugasan CS harus lewat modal (signal $assignOpen), bukan kartu inline")
	}
}

// TestCustomerSuccessDetail_EntryLinks: BL-102 — tombol entry point ke
// Implementation Tracker & Training Schedule dirender saat CanViewX=true + Href
// terisi, mengarah ke daftar ter-filter desa ini (?account={id}). Diuji pada
// empty-state (Exists=false) untuk menegaskan navigasi tak bergantung baris CS.
func TestCustomerSuccessDetail_EntryLinks(t *testing.T) {
	v := CustomerSuccessDetailView{
		Base: "/w/test", ID: 7, AccountName: "Desa X", Exists: false,
		CanViewImplTasks: true, ImplTasksHref: "/w/test/impl-tasks?account=7",
		CanViewTrainings: true, TrainingsHref: "/w/test/trainings?account=7",
	}
	body := renderCSDetail(t, v)
	if !strings.Contains(body, "Implementation Tracker") {
		t.Error("BL-102: CanViewImplTasks=true harus merender tautan Implementation Tracker")
	}
	if !strings.Contains(body, "/w/test/impl-tasks?account=7") {
		t.Error("BL-102: tautan Implementation Tracker harus ter-filter ?account=7")
	}
	if !strings.Contains(body, "Training Schedule") {
		t.Error("BL-102: CanViewTrainings=true harus merender tautan Training Schedule")
	}
	if !strings.Contains(body, "/w/test/trainings?account=7") {
		t.Error("BL-102: tautan Training Schedule harus ter-filter ?account=7")
	}
}

// TestCustomerSuccessDetail_EntryLinksGated: CanViewX=false → tautan onboarding
// tak dirender (gerbang F2 crm:journey).
func TestCustomerSuccessDetail_EntryLinksGated(t *testing.T) {
	v := CustomerSuccessDetailView{
		Base: "/w/test", ID: 7, AccountName: "Desa X", Exists: true,
		CanReadHealth:    true,
		CanViewImplTasks: false, CanViewTrainings: false,
	}
	body := renderCSDetail(t, v)
	if strings.Contains(body, "Implementation Tracker") {
		t.Error("BL-102: CanViewImplTasks=false harus menyembunyikan tautan Implementation Tracker")
	}
	if strings.Contains(body, "Training Schedule") {
		t.Error("BL-102: CanViewTrainings=false harus menyembunyikan tautan Training Schedule")
	}
}

// TestCustomerSuccessDetail_AssignTanpaTulisTersembunyi: CanAssign=false →
// tombol & modal Penugasan CS TAK dirender (aktor bisa baca CS tapi tak berhak
// tulis account).
func TestCustomerSuccessDetail_AssignTanpaTulisTersembunyi(t *testing.T) {
	v := CustomerSuccessDetailView{
		Base: "/w/test", ID: 7, AccountName: "Desa X", Exists: false,
		CanAssign: false,
	}
	body := renderCSDetail(t, v)
	if strings.Contains(body, "Penugasan CS") {
		t.Error("BL-108: CanAssign=false harus menyembunyikan Penugasan CS")
	}
	if strings.Contains(body, "assignOpen") {
		t.Error("BL-108: CanAssign=false tak boleh merender modal Penugasan CS")
	}
}
