package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_activities_singleform_test.go — BL-19: satu form "Tambah Aktivitas"
// dengan dropdown "Jenis" menggantikan 3 tombol/form per-kind. Yang dijaga:
//
//   - Create: kind dari FIELD FORM (bukan URL) → tersimpan sesuai pilihan.
//   - Create: kind tak sah → ?err=activity_kind (bukan diam-diam default).
//   - Edit: kind TERKUNCI — submit kind lain diabaikan (UpdateActivity tak punya
//     kolom Kind; parseActivityForm memakai a.Kind), jenis di DB tak berubah.
//   - Form buat merender dropdown Jenis + toggle Datastar semua grup field.
//   - Daftar menampilkan SATU tombol "Tambah Aktivitas", bukan tautan per-kind.
//
// Koneksi test = superuser (bypass RLS) → uji logika handler; readback lewat
// env.h.Pool (pola lifecycle_state_test.go).

// activityKindBySubject membaca kind satu aktivitas via subjek (superuser, bypass
// RLS) — untuk membuktikan kind yang benar-benar tersimpan.
func (e *testEnv) activityKindBySubject(t *testing.T, subject string) string {
	t.Helper()
	var kind string
	if err := e.h.Pool.QueryRow(t.Context(),
		"SELECT kind FROM activities WHERE tenant_id=$1 AND subject=$2",
		e.tenantID, subject).Scan(&kind); err != nil {
		t.Fatalf("readback kind subjek %q: %v", subject, err)
	}
	return kind
}

// TestActivityCreate_KindFromFormPersists: pilih Jenis=call di form → tersimpan
// sebagai call (kind kini field form, bukan ?kind= URL).
func TestActivityCreate_KindFromFormPersists(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Panggil", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "call")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Panggilan dari Form")
	form.Set("direction", "Outbound")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create call harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.activityKindBySubject(t, "Panggilan dari Form"); got != "call" {
		t.Errorf("kind tersimpan = %q, ingin \"call\"", got)
	}
}

// TestActivityCreate_InvalidKindRejected: kind tak sah dari form → ?err=activity_kind,
// tak tersimpan (validasi validActivityKinds sebelum insert).
func TestActivityCreate_InvalidKindRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "meeting") // belum ber-UI v1 → ditolak
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Kind Tak Sah")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=activity_kind") {
		t.Errorf("kind tak sah harus err=activity_kind, got %q", loc)
	}
}

// TestActivityUpdate_KindLocked: edit aktivitas call sambil menyisipkan kind=note
// di form → jenis tetap "call". kind immutable saat edit (UpdateActivity tanpa
// kolom Kind; parseActivityForm pakai a.Kind).
func TestActivityUpdate_KindLocked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kunci", &uid, nil, nil)
	a := env.seedSalesActivity(t, "call", "account", acc.ID, "Panggilan Asli", &uid)

	form := url.Values{}
	form.Set("kind", "note") // upaya ganti jenis — harus diabaikan
	form.Set("subject", "Panggilan Disunting")
	form.Set("direction", "Inbound")

	req := accountsReq(http.MethodPost, "/activities/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityUpdate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.activityKindBySubject(t, "Panggilan Disunting"); got != "call" {
		t.Errorf("kind setelah edit = %q, ingin tetap \"call\"", got)
	}
}

// TestActivityNew_RendersSingleFormWithKindDropdown: form buat mengeluarkan satu
// dropdown Jenis (name=kind, di-bind $kind) + toggle Datastar untuk SEMUA grup
// field (task/call/note) — bukan lagi satu grup per URL kind.
func TestActivityNew_RendersSingleFormWithKindDropdown(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/activities/new", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityNew status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`name="kind"`, `data-bind="kind"`, `data-signals`,
		`data-show="$kind == `,             // toggle grup per-kind
		"Detail Tugas", "Detail Panggilan", // semua grup dirender sekaligus
	} {
		if !strings.Contains(body, want) {
			t.Errorf("form buat harus memuat %q", want)
		}
	}
}

// TestActivitiesList_SingleAddButton: daftar menampilkan satu tombol "Tambah
// Aktivitas" (BL-19), bukan tautan per-kind (?kind=).
func TestActivitiesList_SingleAddButton(t *testing.T) {
	env, uid := setupAccounts(t)

	body := env.activitiesListBody(t, uid, "member", "sales")
	if !strings.Contains(body, "Tambah Aktivitas") {
		t.Errorf("daftar harus punya tombol \"Tambah Aktivitas\"")
	}
	if strings.Contains(body, "/activities/new?kind=") {
		t.Errorf("tautan buat per-kind (?kind=) tak boleh ada lagi")
	}
}
