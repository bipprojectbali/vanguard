package handler

import (
	"net/url"
	"strings"
	"testing"
)

// sales_deals_stage_sequence_test.go — BL-159: form/POST ubah tahap deal tak
// boleh lompat tahap (sequential-only). Dipisah dari sales_deals_stage_test.go
// (ukuran file) & sales_deals_lost_test.go (Closed Lost picklist, hal berbeda).

// TestNextDealStages_Sequential: nextDealStages murni-fungsi — tiap tahap aktif
// biasa punya TEPAT satu tahap berikutnya; tahap aktif TERAKHIR (Negotiation)
// bercabang dua (Closed Won, Closed Lost — keputusan user 14 Sep: deal hanya
// boleh gugur dari tahap terakhir, bukan dari mana saja); tahap terminal/tak
// dikenal → nil (form disembunyikan di view untuk deal terminal).
func TestNextDealStages_Sequential(t *testing.T) {
	cases := []struct {
		current string
		want    []string
	}{
		{"Prospecting", []string{"Qualification"}},
		{"Qualification", []string{"Demo"}},
		{"Demo", []string{"Proposal"}},
		{"Proposal", []string{"Negotiation"}},
		{"Negotiation", []string{"Closed Won", "Closed Lost"}},
		{"Closed Won", nil},
		{"Closed Lost", nil},
		{"Tahap Ngawur", nil},
	}
	for _, tc := range cases {
		got := nextDealStages(tc.current)
		if len(got) != len(tc.want) {
			t.Errorf("nextDealStages(%q) = %v, want %v", tc.current, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("nextDealStages(%q) = %v, want %v", tc.current, got, tc.want)
				break
			}
		}
	}
}

// TestDealStage_AllowsSequentialNext: pindah tepat SATU tahap ke depan (tahap
// aktif biasa) → sukses (?ok=staged).
func TestDealStage_AllowsSequentialNext(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Maju", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "5000000")

	rec := env.postDealStage(uid, deal.ID, url.Values{"stage": {"Qualification"}})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("pindah 1 tahap ke depan harus sukses, got %q (status %d)\n%s",
			loc, rec.Code, rec.Body.String())
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Qualification" {
		t.Errorf("stage = %q, want Qualification", got.Stage)
	}
}

// TestDealStage_RejectsSkippedStage: lompat lebih dari satu tahap (Prospecting
// → Demo, lewati Qualification) → ditolak backend (?err=stage_sequence), deal
// TETAP di stage lama — jaring terakhir walau POST langsung (bukan lewat UI).
func TestDealStage_RejectsSkippedStage(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Lompat", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "5000000")

	rec := env.postDealStage(uid, deal.ID, url.Values{"stage": {"Demo"}})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("lompat tahap harus ?err=stage_sequence, got %q (status %d)", loc, rec.Code)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Prospecting" {
		t.Errorf("deal tak boleh pindah stage saat lompat ditolak, stage = %q", got.Stage)
	}
}

// TestDealStage_RejectsBackwardMove: mundur satu tahap dari stage aktif (bukan
// reopen dari terminal) juga ditolak — sequential-only berarti HANYA maju satu
// langkah, bukan bebas maju-mundur di antara tahap aktif.
func TestDealStage_RejectsBackwardMove(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Mundur", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Demo", "5000000")

	rec := env.postDealStage(uid, deal.ID, url.Values{"stage": {"Qualification"}})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("mundur tahap harus ?err=stage_sequence, got %q (status %d)", loc, rec.Code)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Demo" {
		t.Errorf("deal tak boleh mundur, stage = %q", got.Stage)
	}
}

// TestDealStage_RejectsClosedLostFromNonLastStage: Closed Lost HANYA sah dari
// tahap aktif terakhir (Negotiation) — dari tahap lebih awal (mis. Qualification)
// ditolak (?err=stage_sequence) meski win_loss_reason & loss_reason_code lengkap
// (isolasi: yang menolak adalah aturan URUTAN, bukan validasi field lain).
func TestDealStage_RejectsClosedLostFromNonLastStage(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Gugur Dini", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Qualification", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Kompetitor"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("Closed Lost dari tahap bukan Negotiation harus ?err=stage_sequence, got %q (status %d)",
			loc, rec.Code)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Qualification" {
		t.Errorf("deal tak boleh Closed Lost dari Qualification, stage = %q", got.Stage)
	}
}

// TestDealStage_AllowsClosedLostFromNegotiation: dari tahap aktif TERAKHIR
// (Negotiation), Closed Lost tetap sah sebagai jalur keluar (bukan hanya
// Closed Won) — regresi memastikan BL-159 tak menutup jalur gugur yang sah.
func TestDealStage_AllowsClosedLostFromNegotiation(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Gugur Wajar", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Harga"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Closed Lost dari Negotiation harus sukses, got %q (status %d)", loc, rec.Code)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Closed Lost" {
		t.Errorf("stage = %q, want Closed Lost", got.Stage)
	}
}
