package handler

import (
	"net/http"
	"strings"
	"testing"
)

// cs_renewals_stage_sequence_test.go — BL-176: state machine transisi
// renewal_stage di CS Renewal Management. Mirror sales_deals_stage_sequence_test.go
// (BL-159), disesuaikan dua hal yang beda dari deal: (1) "" (NULL) dialiaskan
// "Not Started" hanya untuk lookup transisi; (2) Won/Lost mengunci SELURUH form,
// bukan cuma field stage.

// TestNextCSRenewalStages_Sequential: nextCSRenewalStages murni-fungsi — tiap
// tahap aktif bercabang dua (tahap berikutnya, "Lost" — CS bisa gugur di tahap
// mana pun, sama seperti BL-173 pada deal); tahap aktif TERAKHIR (Negotiation)
// → ("Won","Lost"); "" dialiaskan "Not Started"; tahap terminal/tak dikenal → nil.
func TestNextCSRenewalStages_Sequential(t *testing.T) {
	cases := []struct {
		current string
		want    []string
	}{
		{"", []string{"Outreach", "Lost"}},
		{"Not Started", []string{"Outreach", "Lost"}},
		{"Outreach", []string{"Negotiation", "Lost"}},
		{"Negotiation", []string{"Won", "Lost"}},
		{"Won", nil},
		{"Lost", nil},
		{"Tahap Ngawur", nil},
	}
	for _, tc := range cases {
		got := nextCSRenewalStages(tc.current)
		if len(got) != len(tc.want) {
			t.Errorf("nextCSRenewalStages(%q) = %v, want %v", tc.current, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("nextCSRenewalStages(%q) = %v, want %v", tc.current, got, tc.want)
				break
			}
		}
	}
}

// TestIsValidCSRenewalStageTransition: tabel lengkap current×next — semua
// pasangan sah, semua lompat, semua mundur, current==next (no-op, termasuk
// alias "" ≡ "Not Started"), dan current terminal→apa pun (selalu false kecuali
// no-op literal).
func TestIsValidCSRenewalStageTransition(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
		note          string
	}{
		// no-op (termasuk alias "" ≡ "Not Started").
		{"", "", true, "no-op kosong"},
		{"", "Not Started", true, "no-op alias"},
		{"Not Started", "", true, "no-op alias terbalik"},
		{"Outreach", "Outreach", true, "no-op Outreach"},
		{"Won", "Won", true, "no-op terminal Won"},
		{"Lost", "Lost", true, "no-op terminal Lost"},

		// maju sah, satu langkah.
		{"", "Outreach", true, "maju dari kosong"},
		{"Not Started", "Outreach", true, "maju dari Not Started"},
		{"Outreach", "Negotiation", true, "maju dari Outreach"},
		{"Negotiation", "Won", true, "maju ke Won"},

		// gugur (Lost) sah dari tahap aktif mana pun.
		{"", "Lost", true, "gugur dari kosong"},
		{"Outreach", "Lost", true, "gugur dari Outreach"},
		{"Negotiation", "Lost", true, "gugur dari Negotiation"},

		// lompat tahap.
		{"", "Negotiation", false, "lompat dari kosong ke Negotiation"},
		{"Not Started", "Won", false, "lompat dari Not Started ke Won"},
		{"Outreach", "Won", false, "lompat dari Outreach ke Won"},

		// mundur.
		{"Outreach", "Not Started", false, "mundur ke Not Started"},
		{"Outreach", "", false, "mundur/clear ke kosong"},
		{"Negotiation", "Outreach", false, "mundur ke Outreach"},

		// current terminal → apa pun (selain no-op) ditolak.
		{"Won", "Lost", false, "terminal Won → Lost"},
		{"Won", "Outreach", false, "terminal Won → Outreach"},
		{"Won", "", false, "terminal Won → kosong"},
		{"Lost", "Won", false, "terminal Lost → Won"},
		{"Lost", "Negotiation", false, "terminal Lost → Negotiation"},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			got := isValidCSRenewalStageTransition(tc.current, tc.next)
			if got != tc.want {
				t.Errorf("isValidCSRenewalStageTransition(%q, %q) = %v, want %v (%s)",
					tc.current, tc.next, got, tc.want, tc.note)
			}
		})
	}
}

// --- handler: transisi sah/lompat/mundur ------------------------------------

// TestCSRenewalStage_AllowsSequentialNext: POST update dgn stage maju satu
// langkah dari current ("" → "Outreach") → sukses (ok=updated), DB berubah.
func TestCSRenewalStage_AllowsSequentialNext(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Maju CS", &uid)

	form := renewalUpdateForm("Outreach", "Medium")
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Fatalf("maju 1 tahap harus sukses, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RenewalStage == nil || *got.RenewalStage != "Outreach" {
		t.Errorf("renewal_stage = %v, want Outreach", got.RenewalStage)
	}
}

// TestCSRenewalStage_RejectsSkippedStage: lompat dari kosong langsung ke
// Negotiation (lewati Outreach) → ditolak (?err=stage_sequence), DB tak berubah.
func TestCSRenewalStage_RejectsSkippedStage(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Lompat CS", &uid)

	form := renewalUpdateForm("Negotiation", "Medium")
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("lompat tahap harus ?err=stage_sequence, got %q (status %d)", loc, rec.Code)
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RenewalStage != nil {
		t.Errorf("renewal_stage tak boleh berubah saat lompat ditolak, got %v", got.RenewalStage)
	}
}

// TestCSRenewalStage_RejectsBackwardMove: dari "Outreach", mundur ke "Not
// Started" (nilai eksplisit, bukan clear ke "") → ditolak (?err=stage_sequence).
func TestCSRenewalStage_RejectsBackwardMove(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Mundur CS", &uid)
	env.setRenewalStage(t, sub.ID, "Outreach")

	form := renewalUpdateForm("Not Started", "Medium")
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("mundur harus ?err=stage_sequence, got %q (status %d)", loc, rec.Code)
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RenewalStage == nil || *got.RenewalStage != "Outreach" {
		t.Errorf("renewal_stage tak boleh berubah saat mundur ditolak, got %v", got.RenewalStage)
	}
}

// TestCSRenewalStage_RejectsClearToEmpty: dari stage yang sudah terisi
// ("Outreach"), clear balik ke "" (opsi "— Pilih Stage —") diperlakukan sebagai
// mundur biasa → ditolak (?err=stage_sequence), bukan diterima sbg penghapusan.
func TestCSRenewalStage_RejectsClearToEmpty(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Clear CS", &uid)
	env.setRenewalStage(t, sub.ID, "Outreach")

	form := renewalUpdateForm("", "Medium")
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_sequence") {
		t.Fatalf("clear ke kosong harus ?err=stage_sequence, got %q (status %d)", loc, rec.Code)
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RenewalStage == nil || *got.RenewalStage != "Outreach" {
		t.Errorf("renewal_stage tak boleh berubah saat clear ditolak, got %v", got.RenewalStage)
	}
}

// TestCSRenewalStage_SameValueNoOp: submit ulang stage yang sama sebagai current
// (form edit selalu pre-fill) → sukses (no-op sah), meski hanya field lain yang
// berniat diubah.
func TestCSRenewalStage_SameValueNoOp(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa NoOp CS", &uid)
	env.setRenewalStage(t, sub.ID, "Outreach")

	form := renewalUpdateForm("Outreach", "High") // stage sama, cuma risk berubah
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Fatalf("submit ulang stage sama harus sukses, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RenewalRisk == nil || *got.RenewalRisk != "High" {
		t.Errorf("renewal_risk harus tersimpan 'High', got %v", got.RenewalRisk)
	}
}

// --- handler: terminal lock (Won/Lost mengunci SELURUH form) ---------------

// TestCSRenewalStage_TerminalLocksWholeForm: renewal sudah Won/Lost → submit
// APA PUN (termasuk field bukan stage, mis. risk) ditolak (?err=stage_locked),
// DB TAK berubah sama sekali — beda pesan dari stage_sequence.
func TestCSRenewalStage_TerminalLocksWholeForm(t *testing.T) {
	for _, terminal := range []string{"Won", "Lost"} {
		t.Run(terminal, func(t *testing.T) {
			env, uid := setupAccounts(t)
			_, _, sub := env.seedCSRenewal(t, "Desa Terkunci "+terminal, &uid)
			env.setRenewalStage(t, sub.ID, terminal)

			// Submit stage SAMA (no-op andai bukan terminal) + risk baru — tetap
			// harus ditolak krn form terkunci total.
			form := renewalUpdateForm(terminal, "High")
			req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
			rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=stage_locked") {
				t.Fatalf("renewal %s harus ?err=stage_locked, got %q (status %d)", terminal, loc, rec.Code)
			}
			got, err := env.q.GetSubscription(t.Context(), sub.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.RenewalRisk != nil {
				t.Errorf("renewal_risk tak boleh berubah saat form terkunci, got %v", got.RenewalRisk)
			}
		})
	}
}
