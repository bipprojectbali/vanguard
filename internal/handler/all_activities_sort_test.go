package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// all_activities_sort_test.go — BL-157k: feed terpadu /activity-log terurut ke-5
// sumbunya (Jenis/Subjek/Pemilik/Status/Tanggal) via ?sort=col&dir=asc|desc.
// Representatif per pola cursor (mirror sales_activities_sort_test.go BL-157j) +
// kasus KHUSUS union yang tak ada di modul manapun lain: interleave lintas-sumber,
// F3 SERENTAK di dua lengan, paginasi lintas-sumber di bawah sort non-default,
// dan gated-union di bawah ?sort=. Mekanisme cursor generik (dualCursorGen) sudah
// teruji tuntas di all_activities_unified_test.go — tak diulang di sini.

// seedAllActivityFull = varian seedAllActivity (all_activities_test.go) dgn
// kontrol penuh atas status (nullable) — dibutuhkan uji posisi NULL sumbu Status.
// status "" ≡ NULL.
func (e *testEnv) seedAllActivityFull(t *testing.T, kind, ctx, subject string, owner *int64, status string) db.Activity {
	t.Helper()
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	acc := e.seedAccount(t, "Desa-"+subject, owner, nil, nil)
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      "account",
		TargetID:        acc.ID,
		OwnerID:         owner,
		ActivityContext: ptr(ctx),
		Status:          statusPtr,
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity %s (ctx=%s): %v", subject, ctx, err)
	}
	return a
}

// Jenis (kind, NOT NULL, RAW enum alfabetis) — lengan activities saja.
func TestAllActivitiesList_SortKindAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "task", "sales", "AllAct-KindTask", &uid)
	env.seedAllActivity(t, "call", "sales", "AllAct-KindCall", &uid)
	env.seedAllActivity(t, "note", "sales", "AllAct-KindNote", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=kind&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iCall := strings.Index(body, "AllAct-KindCall")
	iNote := strings.Index(body, "AllAct-KindNote")
	iTask := strings.Index(body, "AllAct-KindTask")
	if iCall < 0 || iNote < 0 || iTask < 0 {
		t.Fatalf("ketiga aktivitas harus tampil:\n%s", body)
	}
	if !(iCall < iNote && iNote < iTask) {
		t.Errorf("dir=asc harus urut call < note < task, dapat posisi %d/%d/%d", iCall, iNote, iTask)
	}
}

// Subjek (NOT NULL) asc & desc.
func TestAllActivitiesList_SortSubjectAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekZebra", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekAwal", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekMekar", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "AllAct-SubjekAwal")
	iMekar := strings.Index(body, "AllAct-SubjekMekar")
	iZebra := strings.Index(body, "AllAct-SubjekZebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestAllActivitiesList_SortSubjectDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekZebra2", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekAwal2", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "AllAct-SubjekAwal2")
	iZebra := strings.Index(body, "AllAct-SubjekZebra2")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestAllActivitiesList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "AllAct-Fallback", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AllAct-Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Pemilik (owner_id nullable, kunci sort = ownerName/COALESCE(name,email)).
func TestAllActivitiesList_SortOwnerAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	memA := env.seedMember(t, "allact-owner-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "allact-owner-zebra@x", "member", env.tenantID)

	env.seedAllActivity(t, "note", "sales", "AllAct-OwnerAwal", &memA.ID)
	env.seedAllActivity(t, "note", "sales", "AllAct-OwnerZebra", &memZ.ID)
	env.seedAllActivity(t, "note", "sales", "AllAct-OwnerNull", nil)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=owner&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "AllAct-OwnerAwal")
	iZ := strings.Index(body, "AllAct-OwnerZebra")
	iN := strings.Index(body, "AllAct-OwnerNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// Status (nullable, RAW enum alfabetis).
func TestAllActivitiesList_SortStatusAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivityFull(t, "task", "sales", "AllAct-StatusCompleted", &uid, "Completed")
	env.seedAllActivityFull(t, "task", "sales", "AllAct-StatusInProgress", &uid, "In Progress")
	env.seedAllActivityFull(t, "call", "sales", "AllAct-StatusNull", &uid, "")

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=status&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iC := strings.Index(body, "AllAct-StatusCompleted")
	iP := strings.Index(body, "AllAct-StatusInProgress")
	iN := strings.Index(body, "AllAct-StatusNull")
	if iC < 0 || iP < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iC < iP && iP < iN) {
		t.Errorf("dir=asc harus urut Completed < In Progress < NULL(akhir), dapat posisi %d/%d/%d", iC, iP, iN)
	}
}

// Tanggal (created_at) asc & desc — sumbu ini SUDAH default tanpa ?sort=, test
// ini menempuh jalur ListAllActivitiesSortByDate/ListEngagementsFeedSortByDate
// eksplisit lewat ?sort=date.
func TestAllActivitiesList_SortDateAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "AllAct-TglPertama", &uid)
	env.seedAllActivity(t, "note", "sales", "AllAct-TglKedua", &uid)
	env.seedAllActivity(t, "note", "sales", "AllAct-TglKetiga", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=date&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	i1 := strings.Index(body, "AllAct-TglPertama")
	i2 := strings.Index(body, "AllAct-TglKedua")
	i3 := strings.Index(body, "AllAct-TglKetiga")
	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("ketiga aktivitas harus tampil:\n%s", body)
	}
	if !(i1 < i2 && i2 < i3) {
		t.Errorf("dir=asc harus urut dibuat-lebih-dulu ke lebih-baru, dapat posisi %d/%d/%d", i1, i2, i3)
	}
}

func TestAllActivitiesList_SortDateDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "AllAct-TglA", &uid)
	env.seedAllActivity(t, "note", "sales", "AllAct-TglB", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=date&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "AllAct-TglA")
	iB := strings.Index(body, "AllAct-TglB")
	if iA < 0 || iB < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iB < iA) {
		t.Errorf("dir=desc harus urut lebih-baru dulu: TglB < TglA, dapat posisi %d/%d", iB, iA)
	}
}

// Pagination representatif utk cursor bertipe teks (subject) pada feed terpadu —
// membuktikan dualCursorGen bekerja pada jalur sort, bukan cuma jalur default.
func TestAllActivitiesList_SortSubjectPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastSubject string
	for i := 0; i <= pageSize; i++ {
		subject := "AllAct-PageV" + padded(i)
		env.seedAllActivity(t, "note", "sales", subject, &uid)
		lastSubject = subject
	}

	first := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "admin", first, env.h.AllActivitiesList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastSubject) {
		t.Fatalf("subjek terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/activity-log")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/activity-log?sort=subject&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "admin", second, env.h.AllActivitiesList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastSubject) {
		t.Errorf("subjek terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── Kasus khusus UNION (tak ada di modul BL-157 lain — mekanisme feed terpadu) ──

// TestAllActivitiesList_SortInterleavesBothSources: sumbu Subjek harus meng-
// interleave baris activities & engagements sesuai posisi alfabetis GABUNGAN,
// bukan grup-per-sumber (membuktikan merge-sort lintas-tabel, bukan cuma
// concat dua daftar yang masing-masing terurut).
func TestAllActivitiesList_SortInterleavesBothSources(t *testing.T) {
	env, csm := setupAccounts(t)

	env.seedAllActivity(t, "note", "sales", "Interleave-A-Act", &csm)
	env.seedFeedEngagement(t, "Interleave-B-Eng", nil, &csm)
	env.seedAllActivity(t, "note", "sales", "Interleave-C-Act", &csm)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(csm, "member", "csm", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, mark("Interleave-A-Act"))
	iB := strings.Index(body, mark("Interleave-B-Eng"))
	iC := strings.Index(body, mark("Interleave-C-Act"))
	if iA < 0 || iB < 0 || iC < 0 {
		t.Fatalf("ketiga baris (2 activities, 1 engagement) harus tampil:\n%s", body)
	}
	if !(iA < iB && iB < iC) {
		t.Errorf("dir=asc harus meng-interleave lintas-sumber: A < B(cs) < C, dapat posisi %d/%d/%d", iA, iB, iC)
	}
}

// TestAllActivitiesList_SortRespectsF3BothArms: F3 ownership harus tetap
// berlaku di KEDUA lengan sekaligus di bawah sumbu terurut (bukan cuma jalur
// default) — csm (IsOwn) hanya lihat activities miliknya & engagements desa
// yang ditugaskan padanya, tak lihat milik orang lain di kedua lengan.
func TestAllActivitiesList_SortRespectsF3BothArms(t *testing.T) {
	env, csm := setupAccounts(t)
	other := env.seedMember(t, "f3-both-other@x", "member", env.tenantID)

	env.seedAllActivity(t, "note", "sales", "F3Both-ActMilik", &csm)
	env.seedAllActivity(t, "note", "sales", "F3Both-ActOrang", &other.ID)
	env.seedFeedEngagement(t, "F3Both-EngMilik", nil, &csm)
	env.seedFeedEngagement(t, "F3Both-EngOrang", &other.ID, nil)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(csm, "member", "csm", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, mark("F3Both-ActMilik")) {
		t.Errorf("csm harus melihat activity miliknya (lengan sales) walau sort aktif")
	}
	if strings.Contains(body, mark("F3Both-ActOrang")) {
		t.Errorf("F3 tertembus: csm melihat activity milik orang lain (lengan sales) di bawah sort")
	}
	if !strings.Contains(body, mark("F3Both-EngMilik")) {
		t.Errorf("csm harus melihat engagement desa yang ditugaskan padanya (lengan cs) walau sort aktif")
	}
	if strings.Contains(body, mark("F3Both-EngOrang")) {
		t.Errorf("F3 tertembus: csm melihat engagement desa orang lain (lengan cs) di bawah sort")
	}
}

// TestAllActivitiesList_SortPaginationSpansBothSources: paginasi lintas-tabel
// di bawah sumbu NON-default (subject) — titik risiko tertinggi (cursor
// komposit harus memajukan KEDUA sub-cursor secara independen sesuai sumbu
// aktif, bukan cuma sumbu Tanggal yang sudah diuji CrossSourcePagination).
func TestAllActivitiesList_SortPaginationSpansBothSources(t *testing.T) {
	env, admin := setupAccounts(t)

	const perSource = 13
	var markers []string
	for i := 0; i < perSource; i++ {
		an := "span-act-" + pad3(i)
		env.seedAllActivity(t, "note", "sales", an, &admin)
		markers = append(markers, mark(an))
		en := "span-eng-" + pad3(i)
		env.seedFeedEngagement(t, en, &admin, nil)
		markers = append(markers, mark(en))
	}

	const path = "/w/test/activity-log"
	seen := map[string]int{}
	after := ""
	pages := 0
	for pages < 8 {
		target := path + "?sort=subject&dir=asc"
		if after != "" {
			target += "&after=" + after
		}
		req := accountsReq(http.MethodGet, target, nil, "")
		rec := env.runAccount(admin, "owner", "admin", req, env.h.AllActivitiesList)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
		}
		html := rec.Body.String()
		pages++
		for _, m := range markers {
			if strings.Contains(html, m) {
				seen[m]++
			}
		}
		after = sortAfter(html, path)
		if after == "" {
			break
		}
	}
	if pages < 2 {
		t.Fatalf("dengan %d baris harus ada ≥2 halaman, cuma %d", len(markers), pages)
	}
	for _, m := range markers {
		switch seen[m] {
		case 0:
			t.Errorf("baris %s tak terjangkau lewat paginasi sort=subject (skip)", m)
		case 1: // tepat sekali — benar
		default:
			t.Errorf("baris %s muncul di %d halaman (duplikat lintas-halaman) di bawah sort=subject", m, seen[m])
		}
	}
}

// TestAllActivitiesList_SortGatedUnionExcludesCS: gerbang crm:engagements
// tetap berlaku di bawah ?sort= eksplisit — Sales (tanpa crm:engagements) tak
// melihat baris CS walau sumbu terurut aktif, mirror TestUnifiedFeed_GatedUnion_*
// tapi lewat jalur SortBy* (bukan jalur default).
func TestAllActivitiesList_SortGatedUnionExcludesCS(t *testing.T) {
	env, sales := setupAccounts(t)

	env.seedFeedEngagement(t, "Gated-Sort-Eng", &sales, nil)
	env.seedAllActivity(t, "note", "sales", "Gated-Sort-Act", &sales)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(sales, "member", "sales", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, mark("Gated-Sort-Eng")) {
		t.Errorf("Sales (tanpa crm:engagements) TAK boleh melihat baris CS di bawah ?sort= — bocor gate")
	}
	if !strings.Contains(body, mark("Gated-Sort-Act")) {
		t.Errorf("Sales tetap harus melihat activities miliknya di bawah ?sort= (bukan halaman kosong)")
	}
}
