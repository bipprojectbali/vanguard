package handler

import (
	"net/http"
	"strings"
	"testing"
)

// crm_master_pagination_test.go — BL-6: 4 katalog master (Plans/Playbooks/SLA/
// KB) kini keyset-paginate (created_at DESC, id DESC) lewat ?after=. Sebelumnya
// List*All mengambil SELURUH baris tanpa LIMIT — daftar tumbuh tak berpagar, dan
// baris ke-(pageSize+1) dan seterusnya tak pernah bisa dijangkau lewat UI.
// Kegagalan itu senyap: halaman tampil rapi, isinya saja terpotong. Hanya test
// yang menahannya. Meniru dev_users_page_test.go (pola sama, 4 daftar sekaligus).
//
// Aktor = admin/owner: lolos gate read keempat daftar (plans butuh crm:plans,
// playbooks crm:playbooks, dst — admin punya glob crm:*). Koneksi test =
// superuser (bypass RLS); yang diuji WIRING handler→view (cursor benar diteruskan
// & pager benar dirender), bukan isolasi RLS (itu di rls_test.go).

// masterList = satu daftar master: path, handler-nya, dan cara menyeed satu baris
// yang mengembalikan penanda HTML `>nama<` (teks sel nama di tabel). fn/seed
// diikat ke env di dalam test (perlu env.h / env.q).
type masterList struct {
	name string
	path string // "/w/test/plans"
	fn   func(e *testEnv) http.HandlerFunc
	seed func(t *testing.T, e *testEnv, i int) string
}

// crmMasterLists = keempat daftar. Nama baris "probe-<i>"; penanda `>probe-<i><`
// unik karena batas `<` menutup teks sel (probe-0 bukan awalan probe-01 dst).
var crmMasterLists = []masterList{
	{
		name: "plans", path: "/w/test/plans",
		fn: func(e *testEnv) http.HandlerFunc { return e.h.PlansList },
		seed: func(t *testing.T, e *testEnv, i int) string {
			n := "probe-" + itoa(int64(i))
			e.seedPlanRow(t, n, "PRB-"+itoa(int64(i)), "Core")
			return ">" + n + "<"
		},
	},
	{
		name: "playbooks", path: "/w/test/playbooks",
		fn: func(e *testEnv) http.HandlerFunc { return e.h.PlaybooksList },
		seed: func(t *testing.T, e *testEnv, i int) string {
			n := "probe-" + itoa(int64(i))
			e.seedPlaybookRow(t, n)
			return ">" + n + "<"
		},
	},
	{
		name: "sla", path: "/w/test/sla-policies",
		fn: func(e *testEnv) http.HandlerFunc { return e.h.SLAPoliciesList },
		seed: func(t *testing.T, e *testEnv, i int) string {
			n := "probe-" + itoa(int64(i))
			e.seedSLAPolicyRow(t, n)
			return ">" + n + "<"
		},
	},
	{
		name: "kb", path: "/w/test/kb-articles",
		fn: func(e *testEnv) http.HandlerFunc { return e.h.KBArticlesList },
		seed: func(t *testing.T, e *testEnv, i int) string {
			n := "probe-" + itoa(int64(i))
			e.seedKBArticleRow(t, n)
			return ">" + n + "<"
		},
	},
}

// renderMaster memuat satu daftar sebagai admin/owner & mengembalikan HTML-nya.
// after="" → halaman pertama.
func renderMaster(t *testing.T, e *testEnv, uid int64, l masterList, after string) string {
	t.Helper()
	target := l.path
	if after != "" {
		target += "?after=" + after
	}
	req := accountsReq(http.MethodGet, target, nil, "")
	rec := e.runAccount(uid, "owner", "admin", req, l.fn(e))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: render status %d\n%s", l.name, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// nextAfter memungut cursor dari tautan "Berikutnya" untuk path daftar ini,
// dibaca DARI HTML (bukan dihitung ulang) — yang diuji apakah jalannya benar
// tersedia bagi pembuka halaman. "" bila tak ada tautan (ujung daftar).
func nextAfter(html, path string) string {
	marker := path + "?after="
	i := strings.Index(html, marker)
	if i < 0 {
		return ""
	}
	rest := html[i+len(marker):]
	end := strings.IndexAny(rest, `"'`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// seedPageAndOne menyeed pageSize+1 baris dan mengembalikan penanda baris TERTUA
// (dibuat pertama). Urutan daftar created_at DESC → yang tertua ada di halaman
// belakang, mustahil dijangkau tanpa paginasi yang benar.
func seedPageAndOne(t *testing.T, e *testEnv, l masterList) (oldest string) {
	t.Helper()
	for i := 0; i <= pageSize; i++ {
		m := l.seed(t, e, i)
		if i == 0 {
			oldest = m
		}
	}
	return oldest
}

// TestCRMMaster_BarisTertuaTerjangkauLewatHalamanKedua: INTI. Dengan pageSize+1
// baris, baris tertua HANYA bisa dicapai lewat halaman kedua.
func TestCRMMaster_BarisTertuaTerjangkauLewatHalamanKedua(t *testing.T) {
	for _, l := range crmMasterLists {
		t.Run(l.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			oldest := seedPageAndOne(t, env, l)

			first := renderMaster(t, env, uid, l, "")
			if strings.Contains(first, oldest) {
				t.Fatalf("%s: baris tertua seharusnya belum tampil di halaman pertama", l.name)
			}
			after := nextAfter(first, l.path)
			if after == "" {
				t.Fatalf("%s: halaman pertama harus menawarkan jalan ke berikutnya — "+
					"tanpa itu baris tertua mustahil dijangkau", l.name)
			}
			second := renderMaster(t, env, uid, l, after)
			if !strings.Contains(second, oldest) {
				t.Errorf("%s: baris tertua tak muncul di halaman kedua — daftar masih terpotong", l.name)
			}
		})
	}
}

// TestCRMMaster_HalamanTerakhirMenyatakanUjung: halaman terakhir tak boleh
// menawarkan "Berikutnya" (klik yang berujung kosong) & harus MENGATAKAN bahwa
// itu ujungnya — kalau diam, ia tampak sama dengan tombol yang gagal dirender.
func TestCRMMaster_HalamanTerakhirMenyatakanUjung(t *testing.T) {
	for _, l := range crmMasterLists {
		t.Run(l.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			seedPageAndOne(t, env, l)

			after := nextAfter(renderMaster(t, env, uid, l, ""), l.path)
			last := renderMaster(t, env, uid, l, after)
			if nextAfter(last, l.path) != "" {
				t.Errorf("%s: halaman terakhir tak boleh menawarkan Berikutnya", l.name)
			}
			if !strings.Contains(last, "Ujung daftar") {
				t.Errorf("%s: halaman terakhir harus menyatakan daftar habis", l.name)
			}
		})
	}
}

// TestCRMMaster_CursorRusakTakMengosongkanHalaman: URL bisa disunting/terpotong.
// Cursor rusak harus jatuh ke halaman pertama (splitPage), bukan halaman kosong
// yang terbaca sebagai "tak ada baris sama sekali".
func TestCRMMaster_CursorRusakTakMengosongkanHalaman(t *testing.T) {
	for _, l := range crmMasterLists {
		t.Run(l.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			// Satu baris cukup: yang diuji adalah cursor rusak tetap menampilkannya.
			marker := l.seed(t, env, 0)

			html := renderMaster(t, env, uid, l, "rusak-sekali")
			if !strings.Contains(html, marker) {
				t.Errorf("%s: cursor rusak harus tampilkan halaman pertama, bukan kosong", l.name)
			}
		})
	}
}
