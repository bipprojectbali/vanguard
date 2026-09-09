package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go_starter/internal/appmode"
	"go_starter/internal/db"
	"go_starter/internal/fls"
	"go_starter/internal/session"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/go-chi/chi/v5"
)

// testEnv menampung Handler + session manager untuk test. q = Queries pool-bound
// (seed/assert langsung); tenantID = tenant seed default. Koneksi test = superuser
// (bypass RLS) → uji LOGIKA handler; isolasi RLS sesungguhnya diuji terpisah di
// rls_test.go sebagai role app_rw non-superuser.
type testEnv struct {
	h        *Handler
	sm       *scs.SessionManager
	q        *db.Queries
	tenantID int64
}

// setupTest menyiapkan Handler + tenant/user seed + session manager (memstore,
// bukan Redis). Skip bila TEST_DATABASE_URL kosong. Seed user = OWNER tenant
// default (role tertinggi tenant) agar test aksi admin punya otoritas.
func setupTest(t *testing.T) (*testEnv, int64) {
	t.Helper()
	if pkgPool == nil {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	ctx := context.Background()
	// Pool milik PAKET ini (schema sendiri, lihat main_test.go) — tak dibuka &
	// ditutup per test: membuat schema + migrasi butuh ~1 detik, dan
	// mengulanginya ratusan kali menukar satu masalah kecepatan dengan yang
	// lebih besar.
	pool := pkgPool

	q := db.New(pool)
	// memberships, invites & notifications ikut dibersihkan. RESTART IDENTITY agar
	// id deterministik.
	//
	// TRUNCATE kini AMAN dijalankan paralel dengan paket lain: tabel-tabel ini
	// hidup di schema milik paket handler saja (search_path pool), jadi tak ada
	// data paket lain yang bisa tersentuh.
	if _, err := pool.Exec(ctx, "TRUNCATE notifications, activity_presence, audit_logs, oauth_accounts, invites, memberships, users, tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	// Reset cache FLS phone (BL-107) ke default (kosong). Cache global in-proses TAK
	// tersentuh TRUNCATE; tanpa reset, tenant yang dikonfigurasi satu test bocor ke
	// test berikut (tenantID sama karena RESTART IDENTITY) → default Sales+Admin gagal.
	fls.Load(nil)

	tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{Name: "Test", Slug: "test"})
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	// users kini GLOBAL (tanpa tenant/role); keanggotaan lewat memberships.
	u, err := q.CreateUser(ctx, db.CreateUserParams{Email: "test@local", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: u.ID, TenantID: tenant.ID, Role: "owner",
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	// Session manager pakai memstore agar test tak butuh Redis.
	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	session.Init(sm)

	// Mode DEFAULT untuk test = MULTI, dan itu harus eksplisit sejak 0007.
	// Nilai bawaan paket kini Single (deployment baru lahir sebagai satu
	// aplikasi), sementara seed di atas adalah workspace BIASA — bukan yang
	// primer. Membiarkannya single membuat jalur seperti placeNewUser mencari
	// workspace primer yang tak ada, dan kegagalannya muncul jauh dari sebabnya
	// (register berbalas "email sudah terdaftar").
	//
	// Test yang memang menguji mode single memakai withMode; yang di sini hanya
	// menetapkan latar yang sesuai dengan data seed-nya.
	prevMode := appmode.Current()
	appmode.Set(appmode.Multi)
	t.Cleanup(func() { appmode.Set(prevMode) })

	h := &Handler{Pool: pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return &testEnv{h: h, sm: sm, q: q, tenantID: tenant.ID}, u.ID
}

// seedMember membuat user + membership di tenant tertentu (pola berulang di test
// setelah model membership: user global, keanggotaan terpisah). role default
// "member" bila kosong. tenantID 0 → pakai tenant seed env.
func (e *testEnv) seedMember(t *testing.T, email, role string, tenantID int64) db.User {
	t.Helper()
	ctx := t.Context()
	if tenantID == 0 {
		tenantID = e.tenantID
	}
	if role == "" {
		role = "member"
	}
	u, err := e.q.CreateUser(ctx, db.CreateUserParams{Email: email, PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	if _, err := e.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: u.ID, TenantID: tenantID, Role: role,
	}); err != nil {
		t.Fatalf("seed membership %s: %v", email, err)
	}
	return u
}

// seedUserOnly membuat user TANPA membership — dipakai test yang menguji
// penempatan pendaftar baru (placeNewUser), tempat keanggotaan justru hasil
// yang diuji, bukan prasyarat.
func (e *testEnv) seedUserOnly(t *testing.T, email string) db.User {
	t.Helper()
	u, err := e.q.CreateUser(t.Context(), db.CreateUserParams{Email: email, PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return u
}

// countTenants menghitung workspace yang masih hidup. Query langsung, bukan
// lewat sqlc: hitungan ini hanya dipakai test (untuk membuktikan sesuatu TIDAK
// membuat workspace baru), dan query aplikasi yang tak pernah dipanggil aplikasi
// adalah beban yang menyamar sebagai fitur.
func (e *testEnv) countTenants(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := e.h.Pool.QueryRow(t.Context(),
		"SELECT count(*) FROM tenants WHERE deleted_at IS NULL").Scan(&n); err != nil {
		t.Fatalf("hitung tenants: %v", err)
	}
	return n
}

// sessionCtx membungkus context ber-session aktif (scs butuh request melewati
// LoadAndSave agar Put/Get bekerja).
type sessionCtx struct{ ctx context.Context }

// withSession menjalankan fn dalam context ber-session dengan userID tertentu —
// untuk menguji fungsi yang membaca/menulis session TANPA lewat handler HTTP.
func (e *testEnv) withSession(t *testing.T, userID int64, fn func(sessionCtx)) {
	t.Helper()
	rec := httptest.NewRecorder()
	h := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetUserID(ctx, userID)
		fn(sessionCtx{ctx: ctx})
	}))
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}

// doAuthed menjalankan handler dalam konteks session dengan user login. scs butuh
// request melewati LoadAndSave agar context punya session data; kita bungkus
// handler + inject userID DAN Queries ber-scope (agar h.q(ctx) resolve tanpa
// rantai middleware penuh — koneksi test bypass RLS, jadi pool-bound q cukup).
func (e *testEnv) doAuthed(userID int64, req *http.Request, fn http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID != 0 {
			session.SetUserID(r.Context(), userID)
		}
		fn(w, r.WithContext(withQueries(r.Context(), e.q)))
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// do menjalankan handler tanpa session user (anonim), tetap lewat LoadAndSave.
func (e *testEnv) do(req *http.Request, fn http.HandlerFunc) *httptest.ResponseRecorder {
	return e.doAuthed(0, req, fn)
}

// withChiParam menyisipkan URL param chi (mis. {id}) ke request untuk test.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// ptr mengembalikan pointer ke nilai — untuk field nullable sqlc (mis. PassHash *string).
func ptr[T any](v T) *T { return &v }

// allCatalogPageSize = LIMIT besar untuk helper test yang butuh SELURUH katalog
// master (mis. allPlans/allPlaybooks) lewat query keyset ListXAll — katalog test
// selalu jauh di bawah ambang ini, jadi efektif "ambil semua baris".
const allCatalogPageSize = 1000
