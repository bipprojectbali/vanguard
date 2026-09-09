package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// cs_impl_tasks_account_test.go — BL-102: filter daftar Implementation Tracker
// ke SATU desa lewat ?account={id} (entry point dari halaman Customer Success).
// Menutup: (a) hanya baris desa terpilih tampil + chip konteks, (b) F3 tetap
// mengikat — desa di luar akses aktor → nol baris & NAMA desa tak bocor.

// TestCSImplTasks_AccountFilter: admin membuka /impl-tasks?account={A} → hanya
// task desa A tampil, task desa B tersembunyi, chip "Desa: {nama}" + "Lihat
// semua" dirender. KPI (dicek langsung) ikut menyaring ke desa A.
func TestCSImplTasks_AccountFilter(t *testing.T) {
	env, uid := setupAccounts(t)

	accA := env.seedAccount(t, "Desa Filter A", &uid, nil, nil)
	_ = env.seedCSImplTaskRow(t, accA.ID, "Task Desa A")

	accB := env.seedAccount(t, "Desa Filter B", &uid, nil, nil)
	_ = env.seedCSImplTaskRow(t, accB.ID, "Task Desa B")

	req := accountsReq(http.MethodGet, "/w/test/impl-tasks?account="+itoa(accA.ID), nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTasksList)

	if rec.Code != http.StatusOK {
		t.Fatalf("account filter harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Task Desa A") {
		t.Errorf("?account=A harus menampilkan task desa A")
	}
	if strings.Contains(body, "Task Desa B") {
		t.Errorf("?account=A tak boleh menampilkan task desa B (filter bocor)")
	}
	if !strings.Contains(body, "Desa: Desa Filter A") {
		t.Errorf("chip konteks 'Desa: {nama}' harus dirender saat filter aktif")
	}
	if !strings.Contains(body, "Lihat semua") {
		t.Errorf("tautan 'Lihat semua' (buang filter) harus dirender")
	}

	// KPI ikut menyaring: hanya 1 task desa A, bukan 2 (A+B).
	kpis, err := env.q.CountCSImplTaskKPIs(t.Context(), db.CountCSImplTaskKPIsParams{
		ScopeAll:   true,
		Uid:        &uid,
		UseAccount: true,
		AccountID:  accA.ID,
	})
	if err != nil {
		t.Fatalf("CountCSImplTaskKPIs: %v", err)
	}
	if kpis.TotalCount != 1 {
		t.Errorf("KPI ter-filter desa A harus Total=1, got %d", kpis.TotalCount)
	}
}

// TestCSImplTasks_AccountFilter_F3TakBocor: CSM membuka ?account={desa milik
// orang lain} → 200 tapi NOL baris (F3 fail-closed lewat AND account_id di bawah
// gerbang ownership) dan NAMA desa tak muncul (chip netral tanpa nama) — tak
// membocorkan identitas desa di luar akses.
func TestCSImplTasks_AccountFilter_F3TakBocor(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-acctfilter-task@local", "member", 0).ID

	accOther := env.seedAccount(t, "Desa Rahasia Task", nil, &other, nil)
	_ = env.seedCSImplTaskRow(t, accOther.ID, "Task Rahasia")

	req := accountsReq(http.MethodGet, "/w/test/impl-tasks?account="+itoa(accOther.ID), nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.CSImplTasksList)

	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "Task Rahasia") {
		t.Errorf("CSM tak boleh melihat task desa di luar binaannya (F3 bocor)")
	}
	if strings.Contains(body, "Desa Rahasia Task") {
		t.Errorf("nama desa di luar akses F3 TAK boleh bocor lewat chip")
	}
	if !strings.Contains(body, "Difilter per desa") {
		t.Errorf("chip netral (tanpa nama) harus tetap menandai filter aktif")
	}
}
