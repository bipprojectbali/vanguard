package handler

import (
	"net/url"
	"strings"
	"testing"
)

// sales_deals_lost_test.go — deal Closed Lost (loss_reason_code) & guard
// dropdown status, dipisah dari sales_deals_stage_test.go (ukuran file).

// TestWonSubStatusOptions_MirrorValid: dropdown status (wonSubStatusOptions) WAJIB cermin
// himpunan status yang divalidasi backend (validInitialSubStatuses) — kalau tidak, UI
// menawarkan pilihan yang ditolak, atau menyembunyikan pilihan yang sah. Guard murni-data.
func TestWonSubStatusOptions_MirrorValid(t *testing.T) {
	if len(wonSubStatusOptions) != len(validInitialSubStatuses) {
		t.Fatalf("opsi dropdown (%d) ≠ status valid (%d)",
			len(wonSubStatusOptions), len(validInitialSubStatuses))
	}
	for _, s := range wonSubStatusOptions {
		if _, ok := validInitialSubStatuses[s]; !ok {
			t.Errorf("opsi %q tak ada di validInitialSubStatuses (akan ditolak backend)", s)
		}
	}
}

// ── BL-44 (3a): picklist loss_reason_code saat Closed Lost ───────────────────

// lostForm = form Closed Lost. win_loss_reason tetap wajib (terminal, perilaku
// lama); loss_reason_code opsional di param agar jalur "kosong" bisa diuji.
func lostForm(code string) url.Values {
	f := url.Values{
		"stage":           {"Closed Lost"},
		"win_loss_reason": {"Kalah tender"},
	}
	if code != "" {
		f.Set("loss_reason_code", code)
	}
	return f
}

// TestDealLost_StoresLossReasonCode: Closed Lost dengan kode valid → stage
// tersimpan Closed Lost + loss_reason_code terekam (dipakai grouping laporan).
func TestDealLost_StoresLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kalah", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Kompetitor"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Closed Lost dgn kode valid harus sukses, got %q (status %d)", loc, rec.Code)
	}
	got := env.freshDeal(t, deal.ID)
	if got.Stage != "Closed Lost" {
		t.Errorf("stage = %q, want Closed Lost", got.Stage)
	}
	if got.LossReasonCode == nil || *got.LossReasonCode != "Kompetitor" {
		t.Errorf("loss_reason_code = %v, want Kompetitor", got.LossReasonCode)
	}
}

// TestDealLost_RequiresLossReasonCode: Closed Lost tanpa kode → ?err=loss_reason
// dan deal TETAP stage lama (validasi handler, bukan CHECK DB).
func TestDealLost_RequiresLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Tanpa Kode", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm(""))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=loss_reason") {
		t.Fatalf("kode kosong harus ?err=loss_reason, got %q", loc)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Negotiation" {
		t.Errorf("deal tak boleh pindah stage saat kode kosong, stage = %q", got.Stage)
	}
}

// TestDealLost_RejectsInvalidLossReasonCode: kode di luar picklist ditolak
// (?err=loss_reason) — backend penjaga, bukan sekadar dropdown UI.
func TestDealLost_RejectsInvalidLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kode Ngawur", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Diskon"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=loss_reason") {
		t.Fatalf("kode invalid harus ?err=loss_reason, got %q", loc)
	}
	if got := env.freshDeal(t, deal.ID); got.LossReasonCode != nil {
		t.Errorf("kode invalid tak boleh tersimpan, got %v", got.LossReasonCode)
	}
}

// TestDealReopen_ClearsLossReasonCode: deal Closed Lost (berkode) dibuka kembali
// ke stage aktif → loss_reason_code dibersihkan (kode hanya relevan saat kalah).
func TestDealReopen_ClearsLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reopen", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	if rec := env.postDealStage(uid, deal.ID, lostForm("Anggaran")); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("Closed Lost awal harus sukses, got %q", rec.Header().Get("Location"))
	}
	// Buka kembali ke stage aktif — tak butuh alasan.
	if rec := env.postDealStage(uid, deal.ID, url.Values{"stage": {"Proposal"}}); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("reopen ke Proposal harus sukses, got %q", rec.Header().Get("Location"))
	}
	got := env.freshDeal(t, deal.ID)
	if got.Stage != "Proposal" {
		t.Errorf("stage = %q, want Proposal", got.Stage)
	}
	if got.LossReasonCode != nil {
		t.Errorf("loss_reason_code harus null pasca-reopen, got %v", got.LossReasonCode)
	}
}

// TestLossReasonCodeOptions_MirrorValid: dropdown (lossReasonCodeOptions) WAJIB
// cermin himpunan yang divalidasi backend (validLossReasonCodes) — kalau tidak,
// UI menawarkan pilihan yang ditolak, atau menyembunyikan yang sah.
func TestLossReasonCodeOptions_MirrorValid(t *testing.T) {
	if len(lossReasonCodeOptions) != len(validLossReasonCodes) {
		t.Fatalf("opsi (%d) ≠ kode valid (%d)", len(lossReasonCodeOptions), len(validLossReasonCodes))
	}
	for _, s := range lossReasonCodeOptions {
		if _, ok := validLossReasonCodes[s]; !ok {
			t.Errorf("opsi %q tak ada di validLossReasonCodes (akan ditolak backend)", s)
		}
	}
}
