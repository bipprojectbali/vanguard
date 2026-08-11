package authz

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go_starter/internal/session"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// Sumbu BISNIS (CRM) — enforcer & accessor TERPISAH dari sumbu tenant/platform.
//
// Kenapa dipisah, bukan menambah objek `crm:*` ke enforcer utama: Can() memakai
// session.Role(ctx) (member/admin/owner/staff/super_admin) sebagai subject dan
// short-circuit root untuk super-admin env. Kalau izin CRM ikut di sana,
// owner/super_admin OTOMATIS lolos semua objek CRM lewat god-mode & warisan —
// jadi "pemilik bayangan seluruh desa", dan Field-Level Security tak berarti
// baginya (docs/crm/sistem-dan-role.md §3). Sumbu bisnis ditegakkan atas
// business_role (memberships.business_role), TANPA root, TANPA warisan.
//
// SEJAK 00007 peran bisa DIEDIT per-workspace, jadi policy tak lagi di-embed dari
// CSV: ia hidup di DB (business_role_permissions) dan di-load ke SATU enforcer
// bersama. Agar peran "sales" di workspace A tak memberi izin ke "sales" di
// workspace B, subject di-FOLD jadi `t<tenantID>:<role>` — model tetap 3-field,
// matcher tak berubah, hanya nama subject yang di-namespace-kan. Subject TELANJANG
// (tanpa prefix `t<id>:`) TAK PERNAH di-load; itu akan jadi wildcard lintas-tenant.

// benf = enforcer sumbu bisnis, di-inject via InitBusiness (pola sama seperti enf).
var benf *casbin.SyncedEnforcer

// reloadMu menserialkan swap 3-langkah ReloadBusinessTenant (GetPolicy → Remove →
// Add) agar observably atomik terhadap reload lain. Enforce() sendiri sudah aman
// konkuren lewat RWMutex internal SyncedEnforcer — mutex ini hanya soal urutan
// baca-lalu-tulis pada policy set, bukan proteksi Enforce.
var reloadMu sync.Mutex

// BusinessPerm = satu baris izin (tenant × role × obj × act) sebagaimana dibaca
// dari DB. Tipe milik authz (bukan db) supaya paket ini tetap DB-free — pemanggil
// memetakan baris sqlc ke sini, cermin settings.Load(kv). tenantID + role di-fold
// jadi subject Casbin saat di-load.
type BusinessPerm struct {
	TenantID int64
	Role     string
	Obj      string
	Act      string
}

// foldSubject membentuk subject enforcer ber-namespace tenant: `t<id>:<role>`.
// SATU-SATUNYA tempat subject bisnis dibentuk — dipakai loader DAN CanBusiness,
// agar penulisan & pembacaan tak pernah memakai bentuk berbeda.
func foldSubject(tenantID int64, role string) string {
	return fmt.Sprintf("t%d:%s", tenantID, role)
}

// InitBusiness menyimpan enforcer bisnis global agar CanBusiness bisa dipakai
// handler/middleware. Dipanggil di startup bersama Init.
func InitBusiness(e *casbin.SyncedEnforcer) { benf = e }

// NewBusinessEmpty membangun enforcer bisnis dari BusinessModel dengan NOL policy.
// Policy disuntik terpisah lewat LoadBusiness (dari DB) — bukan lagi dari CSV
// embed. Dipisah dari authz.New karena New mem-parse CSV, sedangkan sumbu bisnis
// kini bersumber DB.
func NewBusinessEmpty() (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(BusinessModel)
	if err != nil {
		return nil, fmt.Errorf("authz: parse business model: %w", err)
	}
	e, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("authz: new business enforcer: %w", err)
	}
	return e, nil
}

// LoadBusiness memuat SELURUH izin (semua tenant) ke enforcer e. Subject di-fold
// `t<id>:<role>`. Dipanggil sekali di startup dengan hasil
// ListAllBusinessRolePermissions — yang WAJIB dibaca lewat WithSuper: di tenant-tx
// RLS menyembunyikan tenant lain, membuat enforcer deny-all senyap bagi mereka.
func LoadBusiness(e *casbin.SyncedEnforcer, perms []BusinessPerm) error {
	rules := make([][]string, 0, len(perms))
	for _, p := range perms {
		rules = append(rules, []string{foldSubject(p.TenantID, p.Role), p.Obj, p.Act})
	}
	if len(rules) == 0 {
		return nil
	}
	if _, err := e.AddPolicies(rules); err != nil {
		return fmt.Errorf("authz: load business policy: %w", err)
	}
	return nil
}

// ReloadBusinessTenant mengganti SELURUH izin satu tenant di enforcer global tanpa
// menyentuh tenant lain — dipanggil setelah matriks peran workspace itu disunting,
// agar perubahan berlaku SEKETIKA tanpa restart. Buang semua policy ber-prefix
// `t<id>:` lalu tanam ulang dari perms. Diserialkan reloadMu.
//
// perms = izin BARU satu tenant (hasil ListBusinessRolePermissionsByTenant).
// Kosong = tenant itu kehilangan seluruh izin (mis. semua peran dihapus) — sah,
// hasilnya deny-default untuk semua peran di sana.
func ReloadBusinessTenant(tenantID int64, perms []BusinessPerm) error {
	if benf == nil {
		return fmt.Errorf("authz: business enforcer belum di-init")
	}
	reloadMu.Lock()
	defer reloadMu.Unlock()

	prefix := fmt.Sprintf("t%d:", tenantID)
	existing, err := benf.GetPolicy()
	if err != nil {
		return fmt.Errorf("authz: baca policy bisnis: %w", err)
	}
	var stale [][]string
	for _, rule := range existing {
		if len(rule) > 0 && strings.HasPrefix(rule[0], prefix) {
			stale = append(stale, rule)
		}
	}
	if len(stale) > 0 {
		if _, err := benf.RemovePolicies(stale); err != nil {
			return fmt.Errorf("authz: buang policy tenant %d: %w", tenantID, err)
		}
	}
	fresh := make([][]string, 0, len(perms))
	for _, p := range perms {
		fresh = append(fresh, []string{foldSubject(p.TenantID, p.Role), p.Obj, p.Act})
	}
	if len(fresh) > 0 {
		if _, err := benf.AddPolicies(fresh); err != nil {
			return fmt.Errorf("authz: tanam policy tenant %d: %w", tenantID, err)
		}
	}
	return nil
}

// CanBusiness melaporkan apakah business_role user (di workspace aktif) boleh
// melakukan act pada objek CRM obj (mis. "crm:tickets", "write").
//
// TAK ADA short-circuit root — sengaja: sumbu bisnis tegak lurus terhadap
// otoritas platform. super_admin yang bukan CSM tetap DITOLAK objek CS; ia
// mengelola sistem, bukan memegang desa. business_role kosong (belum diberi
// peran CRM) → DITOLAK (deny-default). tenant aktif 0 (jalur platform/pre-scope)
// → DITOLAK: tanpa tenant tak ada subject ber-namespace yang sah. Error Enforce =
// fail-CLOSED.
func CanBusiness(ctx context.Context, obj, act string) bool {
	if benf == nil {
		return false
	}
	role := session.BusinessRole(ctx)
	if role == "" {
		return false // belum punya peran CRM di workspace ini
	}
	tenantID := session.TenantID(ctx)
	if tenantID == 0 {
		return false // tanpa tenant aktif tak ada subject ber-namespace
	}
	ok, err := benf.Enforce(foldSubject(tenantID, role), obj, act)
	if err != nil {
		slog.Error("authz business enforce", "obj", obj, "act", act, "err", err)
		return false // fail-closed
	}
	return ok
}

// HasBusinessRole melaporkan apakah tenant punya peran bernama role dengan
// setidaknya satu izin — validasi tenant-aware yang menggantikan switch 5-nama
// tetap (peran kini per-workspace). Peran dengan matriks KOSONG tak muncul di
// enforcer; untuk validasi assign yang otoritatif (peran boleh nol izin), handler
// memakai keberadaan baris business_roles di DB, bukan fungsi ini.
func HasBusinessRole(tenantID int64, role string) bool {
	if benf == nil || role == "" || tenantID == 0 {
		return false
	}
	sub := foldSubject(tenantID, role)
	rules, err := benf.GetFilteredPolicy(0, sub)
	if err != nil {
		return false
	}
	return len(rules) > 0
}

// Nama business_role bawaan — dipakai sebagai nama peran default (di-seed ke tiap
// workspace) DAN sebagai konstanta lintas-paket (fls.go, ownership.go). BUKAN lagi
// daftar tertutup nilai yang sah: peran custom per-workspace juga valid.
const (
	BusinessRoleAdmin   = "admin"
	BusinessRoleManager = "manager"
	BusinessRoleSales   = "sales"
	BusinessRoleCSM     = "csm"
	BusinessRoleSupport = "support"
)
