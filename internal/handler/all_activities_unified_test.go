package handler

import (
	"math"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_unified_test.go — linimasa TERPADU halaman Activities GLOBAL
// (/activity-log, BL-41): activities (Sales/umum) + engagements (CS) dalam SATU
// daftar berkeyset lintas-tabel. Empat sumbu yang dijaga:
//
//   - GATED UNION (BL-41 #1): lengan CS dimuat HANYA bila aktor boleh melihat
//     Engagements (crm:engagements) ATAU platform. Sales/Support → baris CS tak
//     pernah bocor ke /activity-log mereka, BAHKAN untuk engagement di desa yang
//     mereka miliki (gate bukan F3).
//   - VISIBILITAS CS: admin (ScopeAll) & csm (assigned) MELIHAT baris engagement.
//   - F3 per-sumber (BL-41 #2): csm (IsOwn) lihat engagement desa yang ditugaskan
//     padanya, bukan desa orang lain.
//   - PAGINASI KOMPOSIT (BL-41 #3): cursor gabungan dua sumber mengalir lewat
//     jejak BL-7; walk lintas-halaman tak melewatkan/menggandakan baris.

// seedFeedEngagement menaruh satu account + satu engagement di atasnya. owner/
// assignedCSM mengendalikan F3 engagement (EngagementsListFilter menyaring via
// account_owner/assigned_csm/backup_csm).
func (e *testEnv) seedFeedEngagement(
	t *testing.T, subject string, owner, assignedCSM *int64,
) db.Engagement {
	t.Helper()
	acc := e.seedAccount(t, "acc-"+subject, owner, assignedCSM, nil)
	return e.seedEngagementRow(t, acc.ID, subject)
}

// renderUnifiedFeed menjalankan GET /activity-log (opsional ?after=) sebagai
// aktor, mengembalikan HTML shell. Menuntut 200 — gate diuji terpisah.
func (e *testEnv) renderUnifiedFeed(
	t *testing.T, uid int64, wsRole, bizRole, after string,
) string {
	t.Helper()
	target := "/activity-log"
	if after != "" {
		target += "?after=" + after
	}
	req := accountsReq(http.MethodGet, target, nil, "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// mark membungkus teks sel tabel jadi penanda unik `>teks<` — batas `<` menutup
// sel, jadi "eng-1" tak cocok dengan "eng-10" (dan nama account "acc-<subj>" tak
// pernah dirender dengan `>` di depan subjek → nol false-positive).
func mark(s string) string { return ">" + s + "<" }

// TestUnifiedFeed_CSMSeesEngagements: aktor CS (crm:engagements) melihat baris
// engagement di feed global — inti BL-41 (sebelumnya /activity-log hanya query
// activities, engagement CS tak pernah tampil).
func TestUnifiedFeed_CSMSeesEngagements(t *testing.T) {
	env, csm := setupAccounts(t)

	// Engagement di desa yang DITUGASKAN ke csm → lolos F3 IsOwn.
	env.seedFeedEngagement(t, "Eng-CS-Tampil", nil, &csm)
	// Aktivitas Sales milik csm sendiri → juga tampil (lengan activities).
	env.seedAllActivity(t, "note", "sales", "Act-CS-Sales", &csm)

	body := env.renderUnifiedFeed(t, csm, "member", "csm", "")
	if !strings.Contains(body, mark("Eng-CS-Tampil")) {
		t.Errorf("csm harus melihat baris engagement CS di feed terpadu")
	}
	if !strings.Contains(body, mark("Act-CS-Sales")) {
		t.Errorf("csm harus tetap melihat activities miliknya (lengan Sales)")
	}
}

// TestUnifiedFeed_AdminSeesEngagements: admin (ScopeAll) melihat SEMUA engagement
// lintas-owner, tanpa syarat penugasan.
func TestUnifiedFeed_AdminSeesEngagements(t *testing.T) {
	env, admin := setupAccounts(t)
	other := env.seedMember(t, "eng-other@local", "member", 0).ID

	env.seedFeedEngagement(t, "Eng-Milik-Orang", &other, nil)

	body := env.renderUnifiedFeed(t, admin, "owner", "admin", "")
	if !strings.Contains(body, mark("Eng-Milik-Orang")) {
		t.Errorf("admin (ScopeAll) harus melihat engagement lintas-owner")
	}
}

// TestUnifiedFeed_GatedUnion_SalesSupportNoEngagements: KUNCI gated-union. Sales
// & Support (tanpa crm:engagements) TAK melihat baris CS — bahkan engagement di
// desa yang mereka MILIKI. Ini membuktikan gerbang F2 (bukan sekadar F3): jika
// lengan CS dimuat dengan IsOwn, engagement di desa milik sales akan lolos.
func TestUnifiedFeed_GatedUnion_SalesSupportNoEngagements(t *testing.T) {
	env, sales := setupAccounts(t)

	// Engagement di desa yang DIMILIKI sales (account_owner=sales) → F3 IsOwn akan
	// mencocokkannya. Hanya gerbang crm:engagements yang menahannya.
	env.seedFeedEngagement(t, "Eng-Bocor-Sales", &sales, nil)
	env.seedAllActivity(t, "note", "sales", "Act-Sales-Nampak", &sales)

	salesBody := env.renderUnifiedFeed(t, sales, "member", "sales", "")
	if strings.Contains(salesBody, mark("Eng-Bocor-Sales")) {
		t.Errorf("Sales (tanpa crm:engagements) TAK boleh melihat baris CS — bocor gate")
	}
	if !strings.Contains(salesBody, mark("Act-Sales-Nampak")) {
		t.Errorf("Sales tetap harus melihat activities miliknya (bukan halaman kosong)")
	}

	// Support: juga tanpa crm:engagements → lengan CS tak dimuat.
	env.seedFeedEngagement(t, "Eng-Bocor-Support", nil, nil)
	supportBody := env.renderUnifiedFeed(t, sales, "member", "support", "")
	if strings.Contains(supportBody, mark("Eng-Bocor-Support")) {
		t.Errorf("Support (tanpa crm:engagements) TAK boleh melihat baris CS")
	}
}

// TestUnifiedFeed_EngagementF3_IsOwn: F3 per-sumber lengan CS. csm (IsOwn) hanya
// melihat engagement di desa yang ditugaskan padanya, bukan desa orang lain.
func TestUnifiedFeed_EngagementF3_IsOwn(t *testing.T) {
	env, csm := setupAccounts(t)
	other := env.seedMember(t, "f3-other@local", "member", 0).ID

	env.seedFeedEngagement(t, "Eng-Ditugaskan", nil, &csm) // assigned_csm = csm
	env.seedFeedEngagement(t, "Eng-Asing", &other, nil)    // owner other, csm tak terkait

	body := env.renderUnifiedFeed(t, csm, "member", "csm", "")
	if !strings.Contains(body, mark("Eng-Ditugaskan")) {
		t.Errorf("csm harus melihat engagement desa yang ditugaskan padanya")
	}
	if strings.Contains(body, mark("Eng-Asing")) {
		t.Errorf("csm (IsOwn) TAK boleh melihat engagement desa orang lain")
	}
}

// TestUnifiedFeed_CompositeCursorCodec: codec cursor komposit — round-trip murni
// (tak menyentuh DB). Menjaga: sentinel awal "0_0"; token digit_digit (lolos
// ui.validTrailToken); nilai (created_at, id) tiap sumber pulih presisi-nano;
// masukan rusak jatuh ke halaman pertama (fail-open, sejalan pageCursor).
func TestUnifiedFeed_CompositeCursorCodec(t *testing.T) {
	tokenRe := regexp.MustCompile(`^[0-9]+_[0-9]+$`)

	// Halaman pertama → kedua sub-sumber sentinel.
	if got := encodeDualCursor(firstDualCursor()); got != "0_0" {
		t.Errorf("firstDualCursor harus mengkode ke %q, got %q", "0_0", got)
	}
	first := decodeDualCursor("0_0")
	for _, sc := range []subCursor{first.act, first.eng} {
		if sc.at.InfinityModifier != pgtype.Infinity || sc.id != math.MaxInt64 {
			t.Errorf("decode %q harus firstSubCursor (Infinity, MaxInt64), got %+v", "0_0", sc)
		}
	}

	// Baris nyata: round-trip presisi.
	t1 := time.Date(2026, 9, 4, 12, 0, 0, 123, time.UTC)
	t2 := time.Date(2026, 8, 1, 6, 30, 0, 0, time.UTC)
	in := dualCursor{
		act: subCursor{at: pgtype.Timestamptz{Time: t1, Valid: true, InfinityModifier: pgtype.Finite}, id: 42},
		eng: subCursor{at: pgtype.Timestamptz{Time: t2, Valid: true, InfinityModifier: pgtype.Finite}, id: 7},
	}
	tok := encodeDualCursor(in)
	if !tokenRe.MatchString(tok) {
		t.Fatalf("token komposit %q harus digit_digit (lolos validTrailToken)", tok)
	}
	out := decodeDualCursor(tok)
	if out.act.at.Time.UnixNano() != t1.UnixNano() || out.act.id != 42 {
		t.Errorf("sub-cursor act tak pulih: got (%d,%d) want (%d,42)",
			out.act.at.Time.UnixNano(), out.act.id, t1.UnixNano())
	}
	if out.eng.at.Time.UnixNano() != t2.UnixNano() || out.eng.id != 7 {
		t.Errorf("sub-cursor eng tak pulih: got (%d,%d) want (%d,7)",
			out.eng.at.Time.UnixNano(), out.eng.id, t2.UnixNano())
	}

	// Rusak / kosong → halaman pertama (bukan panic / halaman kosong).
	for _, raw := range []string{"", "rusak", "abc_def", "0_0"} {
		dc := decodeDualCursor(raw)
		if dc.act.id != math.MaxInt64 || dc.eng.id != math.MaxInt64 {
			t.Errorf("decode %q harus jatuh ke halaman pertama, got %+v", raw, dc)
		}
	}
}

// TestUnifiedFeed_CrossSourcePagination: INTI paginasi lintas-tabel. Dengan
// >pageSize baris campuran (activities + engagements), walk semua halaman lewat
// cursor komposit yang DIBACA dari HTML — tiap baris terjangkau TEPAT sekali
// (tak ada skip, tak ada duplikat), dan baris tertua hanya lewat halaman kedua.
func TestUnifiedFeed_CrossSourcePagination(t *testing.T) {
	env, admin := setupAccounts(t)

	// Selang-seling dua sumber agar merge lintas-tabel benar-benar teruji.
	// 13 activities + 13 engagements = 26 baris > pageSize(20) → 2 halaman.
	const perSource = 13
	var markers []string
	var oldest string
	for i := 0; i < perSource; i++ {
		an := "uact-" + pad3(i)
		env.seedAllActivity(t, "note", "sales", an, &admin)
		markers = append(markers, mark(an))
		if i == 0 {
			oldest = mark(an) // baris paling awal diseed = tertua (created_at terkecil)
		}
		en := "ueng-" + pad3(i)
		env.seedFeedEngagement(t, en, &admin, nil)
		markers = append(markers, mark(en))
	}

	const path = "/w/test/activity-log"
	seen := map[string]int{}
	after := ""
	pages := 0
	for pages < 8 { // guard: 26 baris / 20 ≤ 2 halaman; 8 = margin aman
		html := env.renderUnifiedFeed(t, admin, "owner", "admin", after)
		pages++
		if pages == 1 && strings.Contains(html, oldest) {
			t.Fatalf("baris tertua tak boleh tampil di halaman pertama (harus terdorong ke hal 2)")
		}
		for _, m := range markers {
			if strings.Contains(html, m) {
				seen[m]++
			}
		}
		after = nextAfter(html, path)
		if after == "" {
			break
		}
	}
	if pages < 2 {
		t.Fatalf("dengan %d baris harus ada ≥2 halaman, cuma %d", len(markers), pages)
	}
	if !strings.Contains(env.renderUnifiedFeed(t, admin, "owner", "admin", ""), markers[len(markers)-1]) {
		// baris terbaru (terakhir diseed) harus di halaman pertama
		t.Errorf("baris terbaru harus tampil di halaman pertama")
	}
	for _, m := range markers {
		switch seen[m] {
		case 0:
			t.Errorf("baris %s tak terjangkau lewat paginasi (skip)", m)
		case 1: // tepat sekali — benar
		default:
			t.Errorf("baris %s muncul di %d halaman (duplikat lintas-halaman)", m, seen[m])
		}
	}
}

// pad3 = zero-pad 3 digit ("0" → "000") agar penanda seragam & mudah dibaca.
func pad3(i int) string {
	s := itoa(int64(i))
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}
