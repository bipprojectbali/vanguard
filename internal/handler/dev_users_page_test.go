package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_starter/internal/session"
)

// dev_users_page_test.go — halaman /dev/users bisa MENJANGKAU seluruh user.
//
// Sebelum ini query-nya sudah keyset tapi halamannya selalu meminta halaman
// pertama dan tak punya kontrol lanjut: user ke-21 dan seterusnya tak bisa
// dicapai sama sekali. Kegagalan seperti itu tak memunculkan error di mana pun —
// halamannya tampil rapi, isinya saja yang tak lengkap. Hanya test yang bisa
// menahannya kembali.

// getDevUsers memanggil halaman sebagai super-admin dan mengembalikan HTML-nya.
// after="" → halaman pertama.
func getDevUsers(t *testing.T, env *testEnv, actorID int64, after string) string {
	t.Helper()
	url := "/dev/users"
	if after != "" {
		url += "?after=" + after
	}
	return getDevUsersURL(t, env, actorID, url)
}

// getDevUsersURL memanggil halaman pada URL apa adanya (termasuk query trail).
// Dipakai saat test perlu MENGIKUTI tautan pager sungguhan (after + trail),
// bukan cuma menyodorkan cursor telanjang.
func getDevUsersURL(t *testing.T, env *testEnv, actorID int64, url string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), actorID, "test@local", "super_admin", true,
			env.tenantID, "Test", "test", "")
		env.h.DevUsersList(w, r.WithContext(withQueries(r.Context(), env.q)))
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec.Body.String()
}

// nextHrefFrom memungut href PENUH tautan "Berikutnya" (after + trail), meng-
// unescape &amp;→& agar bisa langsung dipakai sebagai URL request berikutnya.
// Berbeda dari nextCursorFrom yang hanya mengambil cursor: di sini kita ingin
// membawa jejak agar halaman berikut tahu jalan mundurnya.
func nextHrefFrom(t *testing.T, html string) string {
	t.Helper()
	const marker = `href="/dev/users?after=`
	i := strings.Index(html, marker)
	if i < 0 {
		t.Fatalf("tak ada tautan Berikutnya di halaman")
	}
	rest := html[i+len(`href="`):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		t.Fatalf("tautan berikutnya tak tertutup: %.80s", rest)
	}
	return strings.ReplaceAll(rest[:end], "&amp;", "&")
}

// rowOf = penanda BARIS TABEL milik satu user (id yang dipakai UserRowNode).
//
// Mencari emailnya saja TIDAK cukup: AppShell menampilkan email user yang sedang
// login di sidebar, jadi email aktor selalu ada di HTML halaman mana pun —
// termasuk halaman yang barisnya justru tak memuat dia. Itu membuat pencarian
// naif selalu "ketemu" dan testnya lulus tanpa membuktikan apa-apa.
func rowOf(userID int64) string { return `id="user-` + itoa(userID) + `"` }

// nextCursorFrom memungut cursor dari tautan "Berikutnya" di HTML. Sengaja
// dibaca dari HALAMAN, bukan dihitung ulang di test: yang diuji justru apakah
// jalannya benar-benar tersedia bagi orang yang membuka halaman itu.
func nextCursorFrom(t *testing.T, html string) string {
	t.Helper()
	const marker = `/dev/users?after=`
	i := strings.Index(html, marker)
	if i < 0 {
		return ""
	}
	rest := html[i+len(marker):]
	end := strings.IndexAny(rest, `&"'`)
	if end < 0 {
		t.Fatalf("tautan berikutnya tak tertutup: %.80s", rest)
	}
	return rest[:end]
}

// TestDevUsers_UserKe21Terjangkau: INTI perbaikan. Dengan pageSize+1 user, orang
// terakhir hanya bisa dicapai lewat halaman kedua.
func TestDevUsers_UserKe21Terjangkau(t *testing.T) {
	env, superID := setupTest(t)
	SetSuperAdminChecker(func(email string) bool { return email == "test@local" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	// setupTest sudah menyeed 1 user (aktor). Tambah hingga total pageSize+1.
	// Urutan daftar = created_at DESC, jadi yang DULUAN dibuat ada di halaman
	// belakang — dan aktor sendiri (dibuat paling awal) ikut ke sana.
	for i := range pageSize {
		env.seedMember(t, "orang"+itoa(int64(i))+"@local", "member", 0)
	}

	first := getDevUsers(t, env, superID, "")
	if strings.Contains(first, rowOf(superID)) {
		t.Fatal("prasyarat meleset: user terlama seharusnya belum tampil di halaman pertama")
	}

	next := nextCursorFrom(t, first)
	if next == "" {
		t.Fatal("halaman pertama harus menawarkan jalan ke halaman berikutnya — " +
			"tanpa itu user terlama mustahil dijangkau")
	}

	second := getDevUsers(t, env, superID, next)
	if !strings.Contains(second, rowOf(superID)) {
		t.Error("user ke-21 tak muncul di halaman kedua — daftarnya masih terpotong")
	}
}

// TestDevUsers_HalamanTerakhirMenyatakanUjung: halaman terakhir tak boleh
// menawarkan "Berikutnya" (klik yang berujung kosong), dan harus MENGATAKAN
// bahwa itu ujungnya — kalau diam, ia tampak sama dengan halaman yang tombolnya
// gagal dirender.
func TestDevUsers_HalamanTerakhirMenyatakanUjung(t *testing.T) {
	env, superID := setupTest(t)
	SetSuperAdminChecker(func(email string) bool { return email == "test@local" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	for i := range pageSize {
		env.seedMember(t, "orang"+itoa(int64(i))+"@local", "member", 0)
	}
	// Ikuti tautan Berikutnya SUNGGUHAN (bawa after + trail) agar halaman kedua
	// tahu jalan mundurnya — bukan cuma menyodorkan cursor telanjang.
	last := getDevUsersURL(t, env, superID, nextHrefFrom(t, getDevUsers(t, env, superID, "")))

	if nextCursorFrom(t, last) != "" {
		t.Error("halaman terakhir tak boleh menawarkan Berikutnya")
	}
	if !strings.Contains(last, "Ujung daftar") {
		t.Error("halaman terakhir harus menyatakan bahwa daftarnya habis")
	}
	if !strings.Contains(last, `href="/dev/users"`) {
		t.Error("halaman kedua dst harus punya jalan kembali (« Sebelumnya) ke awal daftar")
	}
}

// TestDevUsers_SatuHalamanTanpaKontrol: daftar yang muat sekali tampil tak perlu
// kontrol navigasi sama sekali — tombol yang tak menuju ke mana pun cuma
// mengundang klik yang tak berbuat apa-apa.
func TestDevUsers_SatuHalamanTanpaKontrol(t *testing.T) {
	env, superID := setupTest(t)
	SetSuperAdminChecker(func(email string) bool { return email == "test@local" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	html := getDevUsers(t, env, superID, "")
	if strings.Contains(html, "Ujung daftar") || nextCursorFrom(t, html) != "" {
		t.Error("daftar satu halaman tak boleh memunculkan kontrol paginasi")
	}
}

// TestDevUsers_CursorRusakTakMengosongkanHalaman: URL bisa disunting atau
// terpotong. Yang rusak harus jatuh ke halaman pertama, bukan ke halaman kosong
// yang terbaca sebagai "tak ada user sama sekali".
func TestDevUsers_CursorRusakTakMengosongkanHalaman(t *testing.T) {
	env, superID := setupTest(t)
	SetSuperAdminChecker(func(email string) bool { return email == "test@local" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	html := getDevUsers(t, env, superID, "rusak-sekali")
	if !strings.Contains(html, rowOf(superID)) {
		t.Error("cursor rusak harus menampilkan halaman pertama, bukan halaman kosong")
	}
}
