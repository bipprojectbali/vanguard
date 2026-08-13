package authz

import (
	"sort"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
)

// business_test.go — bukti sumbu izin BISNIS (CRM) benar-benar tegak lurus
// terhadap sumbu tenant/platform, dan setia pada matriks §4. Yang diuji, bila
// rusak, tak terlihat sampai seseorang membuka modul yang bukan haknya:
//
//   (1) enforcer bisnis TERPISAH — subject = business_role, bukan role tenant.
//   (2) TANPA warisan & TANPA root: super_admin/owner BUKAN otomatis lolos CRM.
//   (3) write mencakup read; approve berdiri sendiri.
//   (4) deny-default: modul di luar matriks role = ditolak.
//   (5) SEJAK 00007 subject di-fold `t<id>:<role>` → izin peran per-workspace,
//       peran "sales" di t1 tak menetes ke "sales" di t2, dan subject TELANJANG
//       (tanpa prefix) tak match apa pun (guard kebocoran wildcard).

// permsForTenant memetakan DefaultBusinessRoles() ke []BusinessPerm milik satu
// tenant — cermin apa yang di-seed migrasi/handler ke DB, lalu di-load enforcer.
func permsForTenant(tenantID int64) []BusinessPerm {
	var out []BusinessPerm
	for _, r := range DefaultBusinessRoles() {
		for _, p := range r.Perms {
			out = append(out, BusinessPerm{TenantID: tenantID, Role: r.Name, Obj: p.Obj, Act: p.Act})
		}
	}
	return out
}

// buildBusinessEnforcer memuat matriks DEFAULT untuk tenant-tenant yang diminta
// ke satu enforcer — persis jalur startup (NewBusinessEmpty + LoadBusiness).
func buildBusinessEnforcer(t *testing.T, tenantIDs ...int64) *casbin.SyncedEnforcer {
	t.Helper()
	e, err := NewBusinessEmpty()
	if err != nil {
		t.Fatalf("NewBusinessEmpty: %v", err)
	}
	var perms []BusinessPerm
	for _, id := range tenantIDs {
		perms = append(perms, permsForTenant(id)...)
	}
	if err := LoadBusiness(e, perms); err != nil {
		t.Fatalf("LoadBusiness: %v", err)
	}
	return e
}

// canFolded meng-enforce dengan subject yang SUDAH di-fold — cermin CanBusiness
// tanpa perlu context/session.
func canFolded(t *testing.T, e *casbin.SyncedEnforcer, tenantID int64, role, obj, act string) bool {
	t.Helper()
	ok, err := e.Enforce(foldSubject(tenantID, role), obj, act)
	if err != nil {
		t.Fatalf("Enforce(t%d:%s,%s,%s): %v", tenantID, role, obj, act, err)
	}
	return ok
}

func TestBusinessPolicy_MatrixEnforcement(t *testing.T) {
	const tid = 1
	e := buildBusinessEnforcer(t, tid)

	cases := []struct {
		role, obj, act string
		want           bool
		why            string
	}{
		// --- Kriteria penerimaan F2: Sales TAK BISA objek ticket-only Support ---
		{"sales", "crm:tickets", "write", false, "sales tak boleh menulis tiket (kolom Sales 6.9 = 👁)"},
		{"sales", "crm:tickets", "read", true, "sales boleh LIHAT tiket desanya"},
		{"support", "crm:tickets", "write", true, "support = pemilik tiket (✓ semua)"},

		// --- Support: HANYA objek tiket. Modul sales/CS ditolak keras ---
		{"support", "crm:leads", "read", false, "support tak boleh leads (kolom Support ✕)"},
		{"support", "crm:deals", "read", false, "support tak boleh deals (✕)"},
		{"support", "crm:success_plans", "read", false, "support tak boleh success plans (✕)"},
		{"support", "crm:accounts", "read", true, "support LIHAT desa saat buka tiket (👁)"},
		{"support", "crm:accounts", "write", false, "support tak boleh UBAH desa (hanya 👁)"},
		{"support", "crm:kb", "write", true, "support tulis knowledge base (✓ tulis)"},

		// --- write mencakup read (matcher), tapi tidak sebaliknya ---
		{"sales", "crm:accounts", "read", true, "write mencakup read"},
		{"sales", "crm:accounts", "write", true, "sales ubah desa yang ditugaskan (◐ubah)"},
		{"csm", "crm:leads", "read", false, "csm nol akses leads (kolom CSM ✕)"},
		{"csm", "crm:quotes", "read", false, "csm nol akses quotes (✕)"},

		// --- approve BERDIRI SENDIRI: Sales buat quote tapi TAK approve diskon ---
		{"sales", "crm:quotes", "write", true, "sales BUAT quote (◐)"},
		{"sales", "crm:deals", "approve", false, "sales TAK approve deal — itu Manager"},
		{"manager", "crm:deals", "approve", true, "manager approve deal (✓ approve)"},
		{"manager", "crm:quotes", "approve", true, "manager approve diskon quote"},
		{"manager", "crm:renewal_mgmt", "approve", true, "manager approve renewal"},

		// --- CSM = pemilik Customer Success; Sales nol di CS ---
		{"csm", "crm:success_plans", "write", true, "csm pemilik success plans (◐✓)"},
		{"csm", "crm:health", "write", true, "csm isi health score (◐✓)"},
		{"sales", "crm:success_plans", "read", false, "sales nol success plans (✕)"},
		{"sales", "crm:engagements", "read", false, "sales nol engagements (✕)"},
		{"sales", "crm:journey", "read", true, "sales lihat journey (👁, docs §4 6.2)"},
		{"sales", "crm:adoption", "read", false, "sales nol product adoption (✕, docs §4 6.4)"},

		// --- Manager lintas-tim tapi Plans hanya 👁, Settings ✕ ---
		{"manager", "crm:accounts", "write", true, "manager ✓ accounts"},
		{"manager", "crm:plans", "read", true, "manager LIHAT plans (👁)"},
		{"manager", "crm:plans", "write", false, "manager TAK ubah plans (hanya 👁)"},

		// --- Admin CRM = ✓ semua modul (glob crm:*) ---
		{"admin", "crm:tickets", "write", true, "admin ✓ semua"},
		{"admin", "crm:leads", "write", true, "admin ✓ semua"},
		{"admin", "crm:deals", "approve", true, "admin ✓ approve semua"},
		{"admin", "crm:roles", "write", true, "admin kelola peran (panel /roles)"},

		// --- TEGAK LURUS: role tenant/platform BUKAN subject sumbu bisnis ---
		{"super_admin", "crm:tickets", "write", false, "super_admin BUKAN business_role — ditolak"},
		{"owner", "crm:accounts", "read", false, "owner tenant tak punya izin CRM otomatis (§3)"},
		{"", "crm:accounts", "read", false, "business_role kosong = deny-default"},
		{"stranger", "crm:accounts", "read", false, "role bisnis tak dikenal ditolak"},
	}
	for _, c := range cases {
		if got := canFolded(t, e, tid, c.role, c.obj, c.act); got != c.want {
			t.Errorf("%s: Enforce(t%d:%s,%s,%s)=%v, want %v",
				c.why, tid, c.role, c.obj, c.act, got, c.want)
		}
	}
}

// TestBusinessNoInheritance menjaga sifat yang paling mudah hilang kalau kelak
// ada yang menambah [role_definition]: sumbu bisnis TAK punya warisan. Admin
// bisnis lolos crm:* lewat GLOB (p, admin, crm:*), BUKAN lewat rantai g — jadi
// menambah role bisnis baru tak diam-diam mewarisi izin admin.
func TestBusinessNoInheritance(t *testing.T) {
	const tid = 1
	e := buildBusinessEnforcer(t, tid)
	if canFolded(t, e, tid, "support", "crm:leads", "read") {
		t.Error("support mewarisi izin sales — sumbu bisnis TAK boleh punya warisan")
	}
	if canFolded(t, e, tid, "sales", "crm:playbooks", "read") {
		t.Error("sales mewarisi izin csm — sumbu bisnis TAK boleh punya warisan")
	}
}

// TestBusinessTenantIsolation — inti 00007: peran senama di dua workspace adalah
// subject BERBEDA. Beri t2 peran "sales" yang HANYA boleh tiket; peran "sales" di
// t1 (default) tetap tak boleh tiket, dan izin t2 tak menetes ke t1.
func TestBusinessTenantIsolation(t *testing.T) {
	e, err := NewBusinessEmpty()
	if err != nil {
		t.Fatalf("NewBusinessEmpty: %v", err)
	}
	perms := permsForTenant(1) // t1: sales default (tiket read-only)
	perms = append(perms,
		BusinessPerm{TenantID: 2, Role: "sales", Obj: "crm:tickets", Act: "write"},
	)
	if err := LoadBusiness(e, perms); err != nil {
		t.Fatalf("LoadBusiness: %v", err)
	}
	// t2:sales BOLEH tulis tiket (matriks kustomnya).
	if !canFolded(t, e, 2, "sales", "crm:tickets", "write") {
		t.Error("t2:sales harus boleh tulis tiket (izin kustom workspace-nya)")
	}
	// t1:sales TETAP tak boleh — izin t2 tak menetes ke t1.
	if canFolded(t, e, 1, "sales", "crm:tickets", "write") {
		t.Error("t1:sales bocor izin dari t2:sales — isolasi subject-folding GAGAL")
	}
	// Dan t2:sales tak mewarisi izin accounts milik t1:sales (matriks kustomnya sempit).
	if canFolded(t, e, 2, "sales", "crm:accounts", "write") {
		t.Error("t2:sales bocor izin accounts dari t1:sales")
	}
}

// TestBusinessBareSubjectNoMatch — subject TELANJANG (tanpa prefix `t<id>:`) tak
// boleh match apa pun. Kalau loader lupa mem-fold, subject jadi wildcard yang
// lolos di SEMUA tenant — kebocoran paling berbahaya, jadi dikunci eksplisit.
func TestBusinessBareSubjectNoMatch(t *testing.T) {
	e := buildBusinessEnforcer(t, 1)
	for _, role := range []string{"admin", "manager", "sales", "csm", "support"} {
		if ok, _ := e.Enforce(role, "crm:accounts", "read"); ok {
			t.Errorf("subject telanjang %q match — harus selalu di-fold t<id>:%s", role, role)
		}
	}
}

// TestReloadBusinessTenant — sunting matriks satu workspace berlaku SEKETIKA tanpa
// menyentuh workspace lain. Reload t2 dengan matriks sempit; t1 tak boleh berubah.
func TestReloadBusinessTenant(t *testing.T) {
	e := buildBusinessEnforcer(t, 1, 2)
	InitBusiness(e)
	t.Cleanup(func() { benf = nil })

	// Awalnya kedua tenant punya default: sales tak boleh tulis tiket.
	if canFolded(t, e, 2, "sales", "crm:tickets", "write") {
		t.Fatal("prasyarat: t2:sales default tak boleh tulis tiket")
	}
	// Reload t2 → sales HANYA boleh tulis tiket.
	err := ReloadBusinessTenant(2, []BusinessPerm{
		{TenantID: 2, Role: "sales", Obj: "crm:tickets", Act: "write"},
	})
	if err != nil {
		t.Fatalf("ReloadBusinessTenant: %v", err)
	}
	if !canFolded(t, e, 2, "sales", "crm:tickets", "write") {
		t.Error("t2:sales harus boleh tulis tiket setelah reload")
	}
	if canFolded(t, e, 2, "sales", "crm:accounts", "write") {
		t.Error("izin lama t2:sales (accounts) harus HILANG setelah replace-all reload")
	}
	// t1 tak tersentuh.
	if !canFolded(t, e, 1, "sales", "crm:accounts", "write") {
		t.Error("reload t2 tak boleh menghapus izin t1:sales")
	}
	if canFolded(t, e, 1, "sales", "crm:tickets", "write") {
		t.Error("reload t2 tak boleh menambah izin t1:sales")
	}
}

// TestReloadBusinessTenant_Empty — reload dengan perms kosong = tenant kehilangan
// SELURUH izin (mis. semua peran dihapus). Sah; hasilnya deny-default.
func TestReloadBusinessTenant_Empty(t *testing.T) {
	e := buildBusinessEnforcer(t, 1, 2)
	InitBusiness(e)
	t.Cleanup(func() { benf = nil })

	if err := ReloadBusinessTenant(2, nil); err != nil {
		t.Fatalf("ReloadBusinessTenant kosong: %v", err)
	}
	if canFolded(t, e, 2, "admin", "crm:accounts", "write") {
		t.Error("t2 harus deny-all setelah reload kosong")
	}
	if !canFolded(t, e, 1, "admin", "crm:accounts", "write") {
		t.Error("reload-kosong t2 tak boleh menyentuh t1")
	}
}

// TestHasBusinessRole — pengganti tenant-aware untuk ValidBusinessRole lama.
// Peran yang punya izin di tenant itu → true; peran asing / tenant lain → false.
func TestHasBusinessRole(t *testing.T) {
	e := buildBusinessEnforcer(t, 1)
	InitBusiness(e)
	t.Cleanup(func() { benf = nil })

	for _, r := range []string{"admin", "manager", "sales", "csm", "support"} {
		if !HasBusinessRole(1, r) {
			t.Errorf("HasBusinessRole(1, %q) harus true", r)
		}
	}
	for _, r := range []string{"", "ceo", "owner", "super_admin", "Sales"} {
		if HasBusinessRole(1, r) {
			t.Errorf("HasBusinessRole(1, %q) harus false", r)
		}
	}
	// Peran ada di t1 tapi TIDAK di t2 (t2 tak di-load).
	if HasBusinessRole(2, "sales") {
		t.Error("HasBusinessRole(2, \"sales\") harus false — t2 tak punya peran apa pun")
	}
}

// TestDefaultRolesMatchLegacyCSV mengunci regresi PALING mahal: matriks efektif
// DefaultBusinessRoles() HARUS setara business_policy.csv lama (satu-satunya beda
// yang DISENGAJA: baris admin `crm:roles write` untuk panel manajemen peran).
// Kalau meleset, M2/M3 (Accounts/Contacts) bisa diam-diam kehilangan akses.
func TestDefaultRolesMatchLegacyCSV(t *testing.T) {
	legacy := parseLegacyMatrix(t, BusinessPolicy)
	got := map[string]bool{}
	for _, r := range DefaultBusinessRoles() {
		for _, p := range r.Perms {
			got[r.Name+"|"+p.Obj+"|"+p.Act] = true
		}
	}
	// Beda yang disengaja: crm:roles write pada admin tak ada di CSV lama.
	delete(got, "admin|crm:roles|write")

	if len(got) != len(legacy) {
		t.Errorf("jumlah izin beda: default=%d legacy=%d", len(got), len(legacy))
	}
	for k := range legacy {
		if !got[k] {
			t.Errorf("izin CSV lama %q HILANG dari DefaultBusinessRoles()", k)
		}
	}
	for k := range got {
		if !legacy[k] {
			t.Errorf("izin %q ADA di DefaultBusinessRoles() tapi tidak di CSV lama", k)
		}
	}
}

// parseLegacyMatrix membaca p-rules dari business_policy.csv embed → set "role|obj|act".
func parseLegacyMatrix(t *testing.T, csv string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, line := range strings.Split(csv, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := splitCSV(line)
		if len(f) < 4 || f[0] != "p" {
			continue
		}
		out[f[1]+"|"+f[2]+"|"+f[3]] = true
	}
	return out
}

// TestDefaultRolesDataScope mengunci pemetaan cakupan F3 bawaan (dibaca
// ownership.go): admin/manager = all, sales/csm = own, support = none.
func TestDefaultRolesDataScope(t *testing.T) {
	want := map[string]string{
		"admin": DataScopeAll, "manager": DataScopeAll,
		"sales": DataScopeOwn, "csm": DataScopeOwn,
		"support": DataScopeNone,
	}
	seen := map[string]bool{}
	for _, r := range DefaultBusinessRoles() {
		if w, ok := want[r.Name]; ok {
			if r.DataScope != w {
				t.Errorf("data_scope %q = %q, want %q", r.Name, r.DataScope, w)
			}
			seen[r.Name] = true
		}
		if r.Name == BusinessRoleAdmin && !r.IsSystem {
			t.Error("admin HARUS is_system (terkunci)")
		}
		if r.Name != BusinessRoleAdmin && r.IsSystem {
			t.Errorf("hanya admin yang is_system, bukan %q", r.Name)
		}
	}
	// Semua nama default harus terlihat sekali (guard nama berubah senyap).
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) != len(want) {
		t.Errorf("peran default = %v, kurang dari yang diharapkan %d", names, len(want))
	}
}
