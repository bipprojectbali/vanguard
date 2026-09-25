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
// biasa bercabang dua (tahap berikutnya, "Closed Lost" — revisi BL-173, keputusan
// user 25 Sep: deal realistis bisa gugur di tahap mana pun, bukan cuma setelah
// march-through penuh); tahap aktif TERAKHIR (Negotiation) bercabang dua juga
// tapi ("Closed Won", "Closed Lost") — "Closed Won" SENGAJA TAK diperluas, tetap
// hanya dari Negotiation; tahap terminal/tak dikenal → nil (form disembunyikan
// di view untuk deal terminal).
func TestNextDealStages_Sequential(t *testing.T) {
	cases := []struct {
		current string
		want    []string
	}{
		{"Prospecting", []string{"Qualification", "Closed Lost"}},
		{"Qualification", []string{"Demo", "Closed Lost"}},
		{"Demo", []string{"Proposal", "Closed Lost"}},
		{"Proposal", []string{"Negotiation", "Closed Lost"}},
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

// TestDealStage_AllowsClosedLostFromAnyActiveStage (BL-173, revisi BL-159):
// Closed Lost SAH dari SETIAP tahap aktif — termasuk Prospecting (cakupan
// penuh, keputusan user 25 Sep) — bukan cuma dari Negotiation. Deal realistis
// bisa gugur di tahap mana pun; march-through penuh sebelum bisa ditandai
// kalah tak sesuai realita sales.
func TestDealStage_AllowsClosedLostFromAnyActiveStage(t *testing.T) {
	for _, stage := range []string{"Prospecting", "Qualification", "Demo", "Proposal"} {
		t.Run(stage, func(t *testing.T) {
			env, uid := setupAccounts(t)
			acc := env.seedAccount(t, "Desa Gugur "+stage, &uid, nil, nil)
			deal := env.seedReportDeal(t, acc.ID, &uid, stage, "5000000")

			rec := env.postDealStage(uid, deal.ID, lostForm("Kompetitor"))
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
				t.Fatalf("Closed Lost dari %s harus sukses (BL-173), got %q (status %d)\n%s",
					stage, loc, rec.Code, rec.Body.String())
			}
			if got := env.freshDeal(t, deal.ID); got.Stage != "Closed Lost" {
				t.Errorf("stage = %q, want Closed Lost", got.Stage)
			}
		})
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
