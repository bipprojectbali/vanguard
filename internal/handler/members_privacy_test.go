package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// members_privacy_test.go — siapa melihat email siapa di halaman anggota.
//
// Halaman ini SENGAJA terbuka untuk semua anggota (0004: beda role = beda AKSI,
// bukan beda ALAMAT) — tahu siapa yang punya akses adalah bagian dari
// mempercayai ruang bersama. Yang dibatasi adalah PII-nya.
//
// Yang dijaga di sini adalah hal yang mustahil dilihat dari layar: email asli
// tak boleh SAMPAI ke browser yang tak berhak. Menyembunyikannya lewat CSS atau
// menyamarkannya di view tetap mengirimkannya, dan di sana ia terbaca di
// view-source meski tak tampak.

// getMembers memanggil halaman anggota sebagai role tertentu dan mengembalikan
// recorder utuh — status DAN body. Statusnya ikut diperiksa karena penolakan
// yang terkirim sebagai 200 tampak seperti sukses bagi apa pun yang membacanya.
func getMembers(t *testing.T, env *testEnv, uid int64, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/w/test/members", nil)
	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "pelihat@local", role, false,
			env.tenantID, "Test", "test", "")
		env.h.MembersPage(w, r.WithContext(withQueries(r.Context(), env.q)))
	})).ServeHTTP(rec, req)
	return rec
}

// renderMembers = getMembers yang hanya mengembalikan HTML. Untuk test yang
// memeriksa ISI halaman bagi yang berhak membukanya.
func renderMembers(t *testing.T, env *testEnv, uid int64, role string) string {
	t.Helper()
	return getMembers(t, env, uid, role).Body.String()
}

// seedMember menambah satu anggota TANPA nama tampilan (akun password dev) dan
// mengembalikan emailnya. Ini jalur FALLBACK: yang tak punya nama masih perlu
// dibedakan satu sama lain, dan di situlah samaran email tetap dipakai.
func seedMember(t *testing.T, env *testEnv, email, role string) string {
	t.Helper()
	u := env.seedUserOnly(t, email)
	if _, err := env.q.CreateMembership(t.Context(), db.CreateMembershipParams{
		UserID: u.ID, TenantID: env.tenantID, Role: role,
	}); err != nil {
		t.Fatalf("seed anggota %s: %v", email, err)
	}
	return email
}

// seedMemberNamed menambah anggota yang PUNYA nama tampilan (jalur normal user
// Google) dan mengembalikan id-nya.
func seedMemberNamed(t *testing.T, env *testEnv, email, name, role string) int64 {
	t.Helper()
	u := env.seedUserOnly(t, email)
	if err := env.q.UpdateUserProfile(t.Context(), db.UpdateUserProfileParams{
		ID: u.ID, Name: ptr(name),
	}); err != nil {
		t.Fatalf("seed nama %s: %v", email, err)
	}
	if _, err := env.q.CreateMembership(t.Context(), db.CreateMembershipParams{
		UserID: u.ID, TenantID: env.tenantID, Role: role,
	}); err != nil {
		t.Fatalf("seed anggota %s: %v", email, err)
	}
	return u.ID
}

// TestMembers_MemberDitolak: GERBANG. Daftar anggota adalah direktori orang —
// nama, wajah, dan keanggotaan setiap orang di satu tempat yang bisa disalin
// sekaligus. Hanya yang mengelola keanggotaan membutuhkannya untuk bekerja.
//
// Yang dijaga di sini bukan sekadar "halamannya tak tampil": tak satu pun data
// anggota boleh SAMPAI ke browser member, sebab di sana ia terbaca di
// view-source meski tak tampak di layar.
func TestMembers_MemberDitolak(t *testing.T) {
	env, uid := setupTest(t)
	seedMemberNamed(t, env, "rekankerja@contoh.com", "Budi Santoso", authz.RoleNameMember)

	// Berlaku di KEDUA mode — pembatasan ini soal siapa yang mengelola
	// keanggotaan, bukan soal bentuk aplikasinya.
	for _, mode := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, mode, func() {
			rec := getMembers(t, env, uid, authz.RoleNameMember)

			if rec.Code != http.StatusForbidden {
				t.Errorf("mode %v: member harus ditolak 403, got %d", mode, rec.Code)
			}
			html := rec.Body.String()
			if strings.Contains(html, "Budi Santoso") {
				t.Errorf("mode %v: NAMA rekan terkirim ke member — direktori bocor", mode)
			}
			if strings.Contains(html, "rekankerja@contoh.com") {
				t.Errorf("mode %v: email rekan terkirim ke member", mode)
			}
			// Penolakan WAJIB dijelaskan + menyebut siapa yang bisa membantu:
			// penolakan telanjang tak bisa ditindaklanjuti penerimanya.
			if !strings.Contains(html, "owner &amp; admin") {
				t.Errorf("mode %v: penolakan harus menyebut siapa yang bisa membukanya", mode)
			}
		})
	}
}

// TestMembers_MemberTakLihatDataDiriSendiriPun: member ditolak SEBELUM query
// dijalankan, jadi barisnya sendiri pun tak ikut terkirim. Ini menandai bahwa
// gerbangnya ada di depan, bukan penyaringan setelah data terlanjur dibaca.
func TestMembers_MemberTakLihatDataDiriSendiriPun(t *testing.T) {
	env, uid := setupTest(t)
	if err := env.q.UpdateMemberRole(t.Context(), db.UpdateMemberRoleParams{
		UserID: uid, TenantID: env.tenantID, Role: authz.RoleNameMember,
	}); err != nil {
		t.Fatalf("turunkan role: %v", err)
	}

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameMember)
		if strings.Contains(html, "test@local") {
			t.Error("halaman penolakan tak boleh memuat data anggota sama sekali")
		}
	})
}

// TestMembers_PengelolaMelihatUtuh: sisi lain. Owner & admin MEMBUTUHKAN alamat
// lengkap (mengundang, mencocokkan orang), jadi penyamaran tak boleh mengenai
// mereka — kalau iya, fitur kelola anggota jadi mustahil dipakai.
func TestMembers_PengelolaMelihatUtuh(t *testing.T) {
	env, uid := setupTest(t)
	rekan := seedMember(t, env, "rekankerja@contoh.com", authz.RoleNameMember)

	// Berlaku di KEDUA mode: yang mengelola keanggotaan harus bisa bekerja, dan
	// mode aplikasi tak mengubah siapa mereka.
	for _, mode := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, mode, func() {
			for _, role := range []string{authz.RoleNameOwner, authz.RoleNameAdmin} {
				rec := getMembers(t, env, uid, role)
				if rec.Code != http.StatusOK {
					t.Errorf("mode %v: %s harus bisa membuka halaman, got %d", mode, role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), rekan) {
					t.Errorf("mode %v: %s harus melihat email utuh — ia yang mengelola", mode, role)
				}
			}
		})
	}
}

// TestMembers_PengelolaMelihatNamaDanEmail: nama TIDAK unik — dua orang bernama
// sama itu lumrah, dan nilainya dikendalikan usernya sendiri di Google. Jadi
// pengelola yang hendak mengeluarkan atau menaikkan seseorang harus tetap punya
// sesuatu yang benar-benar membedakan. Mengganti email dengan nama untuk SEMUA
// orang akan membuat aksi anggota jadi tebak-tebakan.
func TestMembers_PengelolaMelihatNamaDanEmail(t *testing.T) {
	env, uid := setupTest(t)
	seedMemberNamed(t, env, "rekankerja@contoh.com", "Budi Santoso", authz.RoleNameMember)

	withMode(t, appmode.Multi, func() {
		for _, role := range []string{authz.RoleNameOwner, authz.RoleNameAdmin} {
			html := renderMembers(t, env, uid, role)
			if !strings.Contains(html, "Budi Santoso") {
				t.Errorf("%s harus melihat nama", role)
			}
			if !strings.Contains(html, "rekankerja@contoh.com") {
				t.Errorf("%s harus TETAP melihat email — nama tak cukup membedakan orang", role)
			}
		}
	})
}

// TestMembers_NamaDiEscape: nama itu USER-CONTROLLED, dan meski halaman ini kini
// hanya dibuka pengelola, yang MENGISI nama tetap orang biasa. Justru itu yang
// berbahaya: markup dari member biasa akan dieksekusi di halaman orang yang
// punya wewenang penuh atas workspace. Menjaga konvensi "escape semua input
// user" (gotcha #15) tepat di tempat data baru itu masuk.
func TestMembers_NamaDiEscape(t *testing.T) {
	env, uid := setupTest(t)
	seedMemberNamed(t, env, "jahat@contoh.com", `<script>alert(1)</script>`, authz.RoleNameMember)

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameOwner)
		if strings.Contains(html, "<script>alert(1)</script>") {
			t.Error("nama ter-render MENTAH — markup dari satu user masuk ke halaman user lain")
		}
		if !strings.Contains(html, "&lt;script&gt;") {
			t.Error("nama harus tampil dalam bentuk ter-escape, bukan hilang diam-diam")
		}
	})
}

// TestMembers_UndanganTakBocorKeMember: daftar undangan pending memuat email
// ORANG YANG BELUM BERGABUNG plus TAUTAN BERTOKEN. Token itu setara kredensial —
// siapa pun yang memegangnya bisa masuk sebagai penerima undangan.
//
// Kini dijaga gerbang halaman, bukan lagi oleh `canManage` di dalam view. Test
// ini SENGAJA dipertahankan meski tampak berlebihan: ia menguji AKIBAT yang
// harus tetap benar (token tak sampai ke member) tanpa mengasumsikan mekanisme
// mana yang menahannya — jadi ia tetap menangkap kebocoran kalau kelak gerbangnya
// dilonggarkan lagi dan penjagaan kembali bergantung pada view.
func TestMembers_UndanganTakBocorKeMember(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	if _, err := env.q.CreateInvite(ctx, db.CreateInviteParams{
		TenantID: env.tenantID, Email: "calon@contoh.com",
		Role: authz.RoleNameMember, Token: "token-rahasia-uji",
		InvitedBy: &uid,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(inviteTTL), Valid: true},
		Kind:      "internal",
	}); err != nil {
		t.Fatalf("seed undangan: %v", err)
	}

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameMember)
		if strings.Contains(html, "token-rahasia-uji") {
			t.Error("token undangan TAK BOLEH sampai ke member — ia setara kredensial")
		}
		if strings.Contains(html, "calon@contoh.com") {
			t.Error("email calon anggota tak boleh terlihat member")
		}
	})
}
