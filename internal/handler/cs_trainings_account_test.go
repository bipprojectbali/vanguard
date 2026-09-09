package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// cs_trainings_account_test.go — BL-102: filter daftar Training Schedule ke SATU
// desa lewat ?account={id}. Cermin cs_impl_tasks_account_test.go.

// TestCSTrainings_AccountFilter: admin membuka /trainings?account={A} → hanya
// training desa A tampil, chip konteks dirender, KPI ikut menyaring.
func TestCSTrainings_AccountFilter(t *testing.T) {
	env, uid := setupAccounts(t)

	accA := env.seedAccount(t, "Desa Train A", &uid, nil, nil)
	_ = env.seedCSTrainingRow(t, accA.ID, "Training Desa A")

	accB := env.seedAccount(t, "Desa Train B", &uid, nil, nil)
	_ = env.seedCSTrainingRow(t, accB.ID, "Training Desa B")

	req := accountsReq(http.MethodGet, "/w/test/trainings?account="+itoa(accA.ID), nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("account filter harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Training Desa A") {
		t.Errorf("?account=A harus menampilkan training desa A")
	}
	if strings.Contains(body, "Training Desa B") {
		t.Errorf("?account=A tak boleh menampilkan training desa B (filter bocor)")
	}
	if !strings.Contains(body, "Desa: Desa Train A") {
		t.Errorf("chip konteks 'Desa: {nama}' harus dirender saat filter aktif")
	}
	if !strings.Contains(body, "Lihat semua") {
		t.Errorf("tautan 'Lihat semua' (buang filter) harus dirender")
	}

	kpis, err := env.q.CountCSTrainingKPIs(t.Context(), db.CountCSTrainingKPIsParams{
		ScopeAll:   true,
		Uid:        &uid,
		UseAccount: true,
		AccountID:  accA.ID,
	})
	if err != nil {
		t.Fatalf("CountCSTrainingKPIs: %v", err)
	}
	if kpis.TotalCount != 1 {
		t.Errorf("KPI ter-filter desa A harus Total=1, got %d", kpis.TotalCount)
	}
}

// TestCSTrainings_AccountFilter_F3TakBocor: CSM membuka ?account={desa orang
// lain} → nol baris & nama desa tak bocor.
func TestCSTrainings_AccountFilter_F3TakBocor(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-acctfilter-train@local", "member", 0).ID

	accOther := env.seedAccount(t, "Desa Rahasia Train", nil, &other, nil)
	_ = env.seedCSTrainingRow(t, accOther.ID, "Training Rahasia")

	req := accountsReq(http.MethodGet, "/w/test/trainings?account="+itoa(accOther.ID), nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.CSTrainingsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "Training Rahasia") {
		t.Errorf("CSM tak boleh melihat training desa di luar binaannya (F3 bocor)")
	}
	if strings.Contains(body, "Desa Rahasia Train") {
		t.Errorf("nama desa di luar akses F3 TAK boleh bocor lewat chip")
	}
	if !strings.Contains(body, "Difilter per desa") {
		t.Errorf("chip netral (tanpa nama) harus tetap menandai filter aktif")
	}
}
