package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/ui"
)

// regions_search_test.go — BL-163: modal global "Cari Kode Desa/Kecamatan".
// regions GLOBAL (tanpa tenant_id, seed migrasi 00039) — fixture pakai data
// REAL yang sudah ada (villages() dari accounts_test.go, query langsung), bukan
// seed sendiri, sama pola test create akun (BL-66) yang juga butuh region asli.

// regionSearchReq membangun request GET dengan "q" di QUERY STRING (bukan body
// form — handler membacanya dari r.URL.Query(), lihat regions_search.go).
func regionSearchReq(q string) *http.Request {
	return accountsReq(http.MethodGet, "/w/test/regions/search?q="+url.QueryEscape(q), nil, "")
}

// regionSearchDistrictReq membangun request cabang tab "Wilayah" (perluasan
// BL-163): "district" di QUERY STRING, bukan "q".
func regionSearchDistrictReq(district string) *http.Request {
	return accountsReq(http.MethodGet, "/w/test/regions/search?district="+url.QueryEscape(district), nil, "")
}

// codeCellCount menghitung jumlah baris hasil di fragmen — kolom Kode
// SATU-SATUNYA sel bertanda class="font-mono text-xs" (header pakai <th>,
// bukan <td>), jadi hitungannya = jumlah baris data persis, tanpa ambigu
// dengan baris header tabel.
func codeCellCount(body string) int {
	return strings.Count(body, `class="font-mono text-xs"`)
}

// mixedLevelNameFragment menemukan (dari data SEED NYATA, bukan hardcode)
// sebuah fragmen ≥3 karakter yang menjadi PREFIX nama di level 3 (Kecamatan)
// MAUPUN level 4 (Desa) sekaligus — dipakai membuktikan cabang ILIKE nama
// mencampur kedua level dalam SATU query (keputusan desain BL-163), tanpa
// bergantung pada nama tempat tertentu yang bisa berbeda antar seed.
//
// Kandidat prefix-share SAJA tak cukup: SearchRegionsByName men-LIMIT pageSize
// (~20) hasil ber-ORDER BY nama, jadi kandidat dgn ratusan match Desa bisa
// mendorong satu-satunya baris Kecamatan keluar dari halaman pertama. Maka
// tiap kandidat divalidasi lewat query SUNGGUHAN (PageSize sama dgn handler)
// sebelum dipakai — kandidat pertama yang lolos itulah yang dipakai test.
func mixedLevelNameFragment(t *testing.T, env *testEnv) string {
	t.Helper()
	rows, err := env.h.Pool.Query(t.Context(), `
		SELECT a.frag
		FROM (SELECT DISTINCT lower(substr(name, 1, 4)) AS frag FROM regions WHERE level = 3) a
		JOIN (SELECT DISTINCT lower(substr(name, 1, 4)) AS frag FROM regions WHERE level = 4) b
			ON b.frag = a.frag
		WHERE length(a.frag) >= 3
		LIMIT 50
	`)
	if err != nil {
		t.Fatalf("cari kandidat fragmen nama campuran level 3+4: %v (seed regions 00039 tak wajar / TEST_DATABASE_URL salah)", err)
	}
	var candidates []string
	for rows.Next() {
		var frag string
		if err := rows.Scan(&frag); err != nil {
			rows.Close()
			t.Fatalf("scan kandidat fragmen: %v", err)
		}
		candidates = append(candidates, frag)
	}
	rows.Close()

	for _, frag := range candidates {
		dbRows, err := env.q.SearchRegionsByName(t.Context(), db.SearchRegionsByNameParams{
			Pattern: "%" + frag + "%", PageSize: int32(pageSize),
		})
		if err != nil {
			t.Fatalf("SearchRegionsByName kandidat %q: %v", frag, err)
		}
		var hasVillage, hasDistrict bool
		for _, r := range dbRows {
			if r.VillageName == "" {
				hasDistrict = true
			} else {
				hasVillage = true
			}
		}
		if hasVillage && hasDistrict {
			return frag
		}
	}
	t.Fatalf("tak ada kandidat (dari %d) menghasilkan campuran level 3+4 dalam batas pageSize — seed regions 00039 tak wajar", len(candidates))
	return ""
}

// mostCommonNamePrefix menemukan fragmen 3-karakter dengan JUMLAH MATCH
// TERBANYAK di seluruh regions level 3+4 — ~91rb baris (ADR pencarian nama
// tanpa index) menjamin fragmen paling umum > pageSize, dipakai membuktikan
// limit ~20 hasil benar-benar dipangkas, bukan angka yang dihardcode.
func mostCommonNamePrefix(t *testing.T, env *testEnv) string {
	t.Helper()
	var frag string
	err := env.h.Pool.QueryRow(t.Context(), `
		SELECT substr(name, 1, 3) AS frag
		FROM regions
		WHERE level IN (3, 4)
		GROUP BY frag
		ORDER BY count(*) DESC
		LIMIT 1
	`).Scan(&frag)
	if err != nil {
		t.Fatalf("cari fragmen nama paling umum: %v", err)
	}
	return frag
}

// TestRegionSearch_TooShort_NoQuery: ketikan < RegionSearchMinChars → fragmen
// hint DEFAULT ("belum cukup ketikan"), BUKAN hint "nihil" — membuktikan guard
// server-side menahan sebelum sampai ke DB (jika DB tetap dipanggil dgn q
// pendek, ILIKE '%ab%' hampir pasti > 0 baris, hasilnya akan berupa TABEL,
// bukan salah satu dari dua hint ini).
func TestRegionSearch_TooShort_NoQuery(t *testing.T) {
	env, uid := setupAccounts(t)

	req := regionSearchReq("ab") // 2 karakter < RegionSearchMinChars (3)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)

	body := rec.Body.String()
	if strings.Contains(body, "<table") {
		t.Fatalf("ketikan < %d karakter tak boleh memicu query DB (tabel hasil muncul):\n%s", ui.RegionSearchMinChars, body)
	}
	if strings.Contains(body, ui.RegionSearchHintNoResults) {
		t.Fatalf("ketikan pendek tak boleh menampilkan hint \"nihil\" (berarti DB SEMPAT dipanggil):\n%s", body)
	}
}

// TestRegionSearch_ExactCode_Village: kode Kemendagri PERSIS milik satu Desa
// (level 4) → tepat 1 baris, kolom Desa terisi nama desa itu sendiri.
func TestRegionSearch_ExactCode_Village(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	anc, err := env.q.GetRegionAncestry(t.Context(), v.DistrictID)
	if err != nil {
		t.Fatalf("ancestry kecamatan fixture: %v", err)
	}

	req := regionSearchReq(v.Code)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)
	body := rec.Body.String()

	if got := codeCellCount(body); got != 1 {
		t.Fatalf("kode desa persis harus 1 baris, got %d:\n%s", got, body)
	}
	for _, want := range []string{v.Code, v.Name, anc.DistrictName, anc.RegencyName, anc.ProvinceName} {
		if !strings.Contains(body, want) {
			t.Errorf("body tak memuat %q:\n%s", want, body)
		}
	}
}

// TestRegionSearch_ExactCode_District: kode Kemendagri PERSIS milik satu
// Kecamatan (level 3, tanpa Desa) → tepat 1 baris, kolom Desa "-" (kosong).
func TestRegionSearch_ExactCode_District(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	districtCode, err := env.q.GetDistrictCode(t.Context(), v.DistrictID)
	if err != nil {
		t.Fatalf("kode kecamatan fixture: %v", err)
	}
	anc, err := env.q.GetRegionAncestry(t.Context(), v.DistrictID)
	if err != nil {
		t.Fatalf("ancestry kecamatan fixture: %v", err)
	}

	req := regionSearchReq(districtCode)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)
	body := rec.Body.String()

	if got := codeCellCount(body); got != 1 {
		t.Fatalf("kode kecamatan persis harus 1 baris, got %d:\n%s", got, body)
	}
	if !strings.Contains(body, "<td>-</td>") {
		t.Errorf("baris Kecamatan (tanpa Desa) harus render \"-\" di kolom Desa:\n%s", body)
	}
	for _, want := range []string{districtCode, anc.DistrictName, anc.RegencyName, anc.ProvinceName} {
		if !strings.Contains(body, want) {
			t.Errorf("body tak memuat %q:\n%s", want, body)
		}
	}
}

// TestRegionSearch_NameSearch_MixedLevels: pencarian nama (bukan kode)
// mencampur hasil level 3 (Kecamatan) DAN level 4 (Desa) dalam SATU daftar —
// dibuktikan lewat fragmen yang DIJAMIN cocok di kedua level (query DB
// langsung, lihat mixedLevelNameFragment), lalu memastikan SEMUA kode yang
// ditemukan lewat query langsung juga muncul di respons HTTP handler.
func TestRegionSearch_NameSearch_MixedLevels(t *testing.T) {
	env, uid := setupAccounts(t)
	frag := mixedLevelNameFragment(t, env)

	dbRows, err := env.q.SearchRegionsByName(t.Context(), db.SearchRegionsByNameParams{
		Pattern: "%" + frag + "%", PageSize: int32(pageSize),
	})
	if err != nil {
		t.Fatalf("SearchRegionsByName langsung: %v", err)
	}
	var hasVillage, hasDistrict bool
	for _, r := range dbRows {
		if r.VillageName == "" {
			hasDistrict = true
		} else {
			hasVillage = true
		}
	}
	if !hasVillage || !hasDistrict {
		t.Fatalf("fragmen %q tak campuran level 3+4 (village=%v district=%v) — cek mixedLevelNameFragment", frag, hasVillage, hasDistrict)
	}

	req := regionSearchReq(frag)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)
	body := rec.Body.String()

	if got := codeCellCount(body); got != len(dbRows) {
		t.Fatalf("jumlah baris respons (%d) tak cocok query DB langsung (%d)", got, len(dbRows))
	}
	for _, r := range dbRows {
		if !strings.Contains(body, r.Code) {
			t.Errorf("respons HTTP tak memuat kode %q dari query DB langsung", r.Code)
		}
	}
}

// TestRegionSearch_NameSearch_LimitEnforced: fragmen dgn match TERBANYAK di
// seluruh dataset (ditemukan dinamis) tetap dipangkas ke pageSize (~20) baris.
func TestRegionSearch_NameSearch_LimitEnforced(t *testing.T) {
	env, uid := setupAccounts(t)
	frag := mostCommonNamePrefix(t, env)

	var total int
	if err := env.h.Pool.QueryRow(t.Context(),
		`SELECT count(*) FROM regions WHERE level IN (3, 4) AND name ILIKE $1`,
		"%"+frag+"%",
	).Scan(&total); err != nil {
		t.Fatalf("hitung total match fragmen %q: %v", frag, err)
	}
	if total <= pageSize {
		t.Fatalf("fragmen %q cuma %d match (butuh > %d agar limit teruji) — seed regions berubah?", frag, total, pageSize)
	}

	req := regionSearchReq(frag)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)

	if got := codeCellCount(rec.Body.String()); got != pageSize {
		t.Errorf("hasil harus dipangkas ke %d baris (dari %d match), got %d", pageSize, total, got)
	}
}

// TestRegionSearch_District_ReturnsDistrictAndVillages: cabang baru tab
// "Wilayah" (perluasan BL-163) — district=<id> Kecamatan sah harus
// menghasilkan baris Kecamatan itu sendiri (Desa "-") + semua Desa anaknya,
// dicocokkan terhadap query DB langsung (SearchRegionsByDistrict) sama pola
// TestRegionSearch_NameSearch_MixedLevels.
func TestRegionSearch_District_ReturnsDistrictAndVillages(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	dbRows, err := env.q.SearchRegionsByDistrict(t.Context(), db.SearchRegionsByDistrictParams{
		DistrictID: v.DistrictID, PageSize: int32(pageSize),
	})
	if err != nil {
		t.Fatalf("SearchRegionsByDistrict langsung: %v", err)
	}
	if len(dbRows) == 0 {
		t.Fatalf("kecamatan fixture %d harus >=1 baris (dirinya sendiri)", v.DistrictID)
	}

	req := regionSearchDistrictReq(strconv.FormatInt(v.DistrictID, 10))
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)
	body := rec.Body.String()

	if got := codeCellCount(body); got != len(dbRows) {
		t.Fatalf("jumlah baris respons (%d) tak cocok query DB langsung (%d)", got, len(dbRows))
	}
	for _, r := range dbRows {
		if !strings.Contains(body, r.Code) {
			t.Errorf("respons HTTP tak memuat kode %q dari query DB langsung", r.Code)
		}
	}
}

// TestRegionSearch_District_Prioritized_OverQ: district= DICEK LEBIH DULU
// (komentar RegionSearch) — "q" basi yang tersisa di query string harus
// diabaikan bila district juga terisi.
func TestRegionSearch_District_Prioritized_OverQ(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	req := accountsReq(http.MethodGet,
		"/w/test/regions/search?q=xx&district="+url.QueryEscape(strconv.FormatInt(v.DistrictID, 10)), nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)
	body := rec.Body.String()

	// "xx" (2 karakter) < RegionSearchMinChars — bila jalur "q" yang dipakai,
	// hint akan "belum cukup ketikan", BUKAN tabel hasil kecamatan.
	if !strings.Contains(body, "<table") {
		t.Errorf("district= harus diprioritaskan drpd q= basi (harus tabel hasil, bukan hint):\n%s", body)
	}
}

// TestRegionSearch_District_Invalid_400: district bukan int64 sah → 400
// (bukan fallback diam-diam ke jalur q).
func TestRegionSearch_District_Invalid_400(t *testing.T) {
	env, uid := setupAccounts(t)

	req := regionSearchDistrictReq("bukan-angka")
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("district tak valid harus 400, got %d", rec.Code)
	}
}

// TestRegionSearch_District_NonNavigating: sama kontrak SSE fragmen dgn
// cabang "q" (TestRegionSearch_NonNavigating) — modal TETAP TERBUKA.
func TestRegionSearch_District_NonNavigating(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	req := regionSearchDistrictReq(strconv.FormatInt(v.DistrictID, 10))
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)

	if rec.Code != http.StatusOK {
		t.Errorf("respons harus 200 (fragmen SSE), got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type harus text/event-stream (fragmen, bukan navigasi), got %q", ct)
	}
}

// TestRegionSearch_NonNavigating: respons SSE fragment (200 + text/event-stream),
// BUKAN redirect/navigasi — modal TETAP TERBUKA di sisi client (gotcha #16:
// sse.Redirect diblokir CSP, jadi kontrak ini WAJIB fragmen, tak pernah 303).
func TestRegionSearch_NonNavigating(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	req := regionSearchReq(v.Code)
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionSearch)

	if rec.Code != http.StatusOK {
		t.Errorf("respons harus 200 (fragmen SSE), got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type harus text/event-stream (fragmen, bukan navigasi), got %q", ct)
	}
}
