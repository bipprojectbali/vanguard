package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// tsAt membungkus waktu jadi timestamptz sqlc (pola berulang di test paginasi).
func tsAt(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// notifications_page_test.go — HALAMAN notifikasi bisa menjangkau seluruh
// riwayat.
//
// Bedanya dengan TestNotif_KeysetPaginasi: yang itu menguji QUERY-nya bisa
// dipaginasi, ini menguji halamannya benar-benar MENAWARKAN jalan itu kepada
// orang yang membukanya. Query yang mampu tapi tak pernah diminta lanjut adalah
// persis keadaan sebelum perbaikan ini — dan ia tak menghasilkan error di mana
// pun, cuma riwayat yang diam-diam terpotong di baris ke-20.

// seedNotifAt menyisipkan notifikasi dengan created_at eksplisit, agar test bisa
// mengurutkan lebih dari satu halaman secara deterministik.
func seedNotifAt(t *testing.T, env *testEnv, userID int64, kind string, at time.Time) {
	t.Helper()
	_, err := env.h.Pool.Exec(t.Context(),
		`INSERT INTO notifications (user_id, tenant_id, kind, payload, created_at)
		 VALUES ($1, $2, $3, '{}'::jsonb, $4)`,
		userID, env.tenantID, kind, at)
	if err != nil {
		t.Fatalf("seed notifikasi: %v", err)
	}
}

// getNotifPage membuka halaman notifikasi dan mengembalikan HTML-nya.
func getNotifPage(t *testing.T, env *testEnv, uid int64, query string) string {
	t.Helper()
	url := "/notifications"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := env.doAuthed(uid, req, func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		env.h.NotificationsPage(w, r)
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("halaman notifikasi status %d", rec.Code)
	}
	return rec.Body.String()
}

// notifCursorFrom memungut cursor dari tautan "Lebih lama" di HALAMAN — bukan
// dirakit ulang di test: yang diuji justru apakah jalannya tersedia bagi yang
// membuka halaman itu.
func notifCursorFrom(t *testing.T, html string) string {
	t.Helper()
	const marker = `/notifications?after=`
	i := strings.Index(html, marker)
	if i < 0 {
		return ""
	}
	rest := html[i+len(marker):]
	end := strings.IndexAny(rest, `&"'`)
	if end < 0 {
		t.Fatalf("tautan berikutnya tak tertutup: %.60s", rest)
	}
	return rest[:end]
}

// TestNotifPage_PeristiwaKe21Terjangkau: INTI perbaikan. Yang paling LAMA justru
// yang dicari saat seseorang bertanya "kapan role saya diubah?" — dan itulah
// yang dulu mustahil dijangkau.
func TestNotifPage_PeristiwaKe21Terjangkau(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	seedNotifAt(t, env, uid, "workspace.joined", now.Add(-time.Duration(notifPageSize+1)*time.Minute))
	for i := range notifPageSize {
		seedNotifAt(t, env, uid, "member.role.changed", now.Add(-time.Duration(i)*time.Minute))
	}

	first := getNotifPage(t, env, uid, "")
	if strings.Contains(first, "bergabung ke") {
		t.Fatal("prasyarat meleset: peristiwa terlama seharusnya belum tampil di halaman pertama")
	}
	next := notifCursorFrom(t, first)
	if next == "" {
		t.Fatal("halaman pertama harus menawarkan jalan ke peristiwa lebih lama")
	}
	if second := getNotifPage(t, env, uid, "after="+next); !strings.Contains(second, "bergabung ke") {
		t.Error("notifikasi ke-21 tak terjangkau — riwayatnya masih terpotong")
	}
}

// TestNotifPage_SatuHalamanTanpaKontrol: riwayat yang muat sekali tampil tak
// perlu kontrol navigasi — tombol yang tak menuju ke mana pun cuma mengundang
// klik yang tak berbuat apa-apa.
func TestNotifPage_SatuHalamanTanpaKontrol(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")

	html := getNotifPage(t, env, uid, "")
	if notifCursorFrom(t, html) != "" {
		t.Error("riwayat satu halaman tak boleh menawarkan 'Lebih lama'")
	}
	if strings.Contains(html, "Ujung riwayat") {
		t.Error("kontrol paginasi tak boleh muncul sama sekali saat hanya ada satu halaman")
	}
}

// TestNotifPage_UndanganHanyaDiHalamanPertama: undangan adalah TUGAS, bukan
// riwayat. Mengulanginya di tiap halaman membuat orang mengira undangannya
// bertambah setiap kali menekan "lebih lama".
func TestNotifPage_UndanganHanyaDiHalamanPertama(t *testing.T) {
	env, uid := setupTest(t)
	env.mkInvite(t, "tok-sekali", "test@local", "member", time.Hour)
	now := time.Now()
	for i := range notifPageSize + 1 {
		seedNotifAt(t, env, uid, "member.role.changed", now.Add(-time.Duration(i)*time.Minute))
	}

	first := getNotifPage(t, env, uid, "")
	if !strings.Contains(first, "tok-sekali") {
		t.Fatal("undangan harus tampil di halaman pertama")
	}
	second := getNotifPage(t, env, uid, "after="+notifCursorFrom(t, first))
	if strings.Contains(second, "tok-sekali") {
		t.Error("undangan tak boleh diulang di halaman berikutnya — ia tugas, bukan riwayat")
	}
}

// TestNotifPage_HalamanKosongTakBerbohong: halaman kedua yang kosong tak boleh
// berkata "belum ada notifikasi" kepada orang yang baru saja melihat daftarnya
// di halaman sebelumnya — dan wajib menyediakan jalan kembali.
func TestNotifPage_HalamanKosongTakBerbohong(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")

	// Cursor sah tapi menunjuk jauh ke masa lalu → tak ada baris tersisa.
	old := formatCursor(tsAt(time.Now().Add(-365*24*time.Hour)), 1)
	html := getNotifPage(t, env, uid, "after="+old)

	if strings.Contains(html, "Belum ada notifikasi") {
		t.Error("halaman kedua yang kosong tak boleh berkata riwayatnya kosong")
	}
	if !strings.Contains(html, "/notifications\"") {
		t.Error("halaman kosong wajib menyediakan jalan kembali ke yang terbaru")
	}
}

// TestNotifPage_CursorRusakTakMengosongkan: URL bisa disunting atau terpotong.
func TestNotifPage_CursorRusakTakMengosongkan(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")

	html := getNotifPage(t, env, uid, "after=rusak-sekali")
	if !strings.Contains(html, "Role Anda") {
		t.Error("cursor rusak harus menampilkan halaman pertama, bukan halaman kosong")
	}
}

// TestNotifPage_TakBocorAntarUserSaatPaginasi: `WHERE user_id` adalah
// SATU-SATUNYA penjaga isolasi di tabel ini (notifications sengaja tanpa RLS),
// jadi jalur baru apa pun yang membacanya wajib diuji ulang — bukan diasumsikan
// aman karena jalur lamanya aman.
func TestNotifPage_TakBocorAntarUserSaatPaginasi(t *testing.T) {
	env, uid := setupTest(t)
	lain := env.seedMember(t, "oranglain@local", "member", 0)
	now := time.Now()
	for i := range notifPageSize + 1 {
		seedNotifAt(t, env, uid, "member.role.changed", now.Add(-time.Duration(i)*time.Minute))
	}
	// Milik orang lain, ditaruh PALING LAMA agar ia yang akan muncul lebih dulu
	// bila filter user_id sampai bocor di halaman kedua.
	seedNotifAt(t, env, lain.ID, "member.removed", now.Add(-999*time.Minute))

	first := getNotifPage(t, env, uid, "")
	second := getNotifPage(t, env, uid, "after="+notifCursorFrom(t, first))
	if strings.Contains(second, "dikeluarkan dari") {
		t.Error("notifikasi milik user lain bocor ke halaman kedua")
	}

	// Sebaliknya: yang bersangkutan tetap melihat miliknya sendiri.
	if rows, err := env.q.ListNotifications(t.Context(), db.ListNotificationsParams{
		UserID:          lain.ID,
		CursorCreatedAt: tsAt(time.Now().Add(time.Hour)), CursorID: 1 << 62,
		PageSize: 10,
	}); err != nil || len(rows) != 1 {
		t.Errorf("pemilik sah harus melihat notifikasinya (rows=%d, err=%v)", len(rows), err)
	}
}
