// Package fls menyimpan kebijakan Field-Level Security phone (HP/WhatsApp) yang
// bisa dikonfigurasi per-tenant (BL-107, ADR 0012), sebagai cache in-proses.
//
// Kenapa cache, bukan baca DB per-request: maskPhone dipanggil sekali PER BARIS di
// daftar Kontak/Lead — baca DB tiap kali = N+1, dan view-mapper bebas-fungsi tak
// punya tempat menyimpan satu-kali-muat. Cache memberi baca O(1) di bawah RLock,
// benar-per-request pada instance yang melayani, & isolasi tenant struktural
// (berkunci tenantID). Ini menyandang caveat eventual-consistency antar-instance
// yang SAMA dengan internal/settings & enforcer bisnis F2: penulis tunggal = admin
// workspace, kasus terburuk jendela singkat di instance non-pelayan sampai reload/
// boot berikutnya. Bila keseragaman lintas-instance seketika kelak wajib, jalur
// pub/sub Redis yang sudah ada berlaku (di luar cakupan).
//
// Paket ini DB-free (seperti internal/authz): baris mentah dipetakan ke Row di
// paket main lalu dimuat ke sini. Impor authz HANYA untuk konstanta nama peran,
// jadi DEFAULT terkunci hidup di satu tempat; authz tak mengimpor fls (tanpa siklus).
package fls

import (
	"sync"

	"go_starter/internal/authz"
)

// Policy = kebijakan FLS phone untuk satu (tenant, business_role).
type Policy struct {
	CanViewPhone bool
	CanEditPhone bool
}

// Row = satu baris kebijakan mentah, dipetakan dari sqlc di paket main agar paket
// ini tetap DB-free (pola authz.BusinessPerm).
type Row struct {
	TenantID     int64
	BusinessRole string
	CanViewPhone bool
	CanEditPhone bool
}

var (
	mu sync.RWMutex
	// byTenant[tenantID] ADA = tenant TERKONFIGURASI (pakai barisnya, peran tanpa
	// baris → fail-closed). ABSEN = belum dikonfigurasi → DEFAULT terkunci.
	byTenant = map[int64]map[string]Policy{}
)

// Load mengganti SELURUH isi cache (dipanggil sekali saat startup dari semua tenant).
func Load(rows []Row) {
	m := group(rows)
	mu.Lock()
	byTenant = m
	mu.Unlock()
}

// ReloadTenant mengganti kebijakan SATU tenant setelah admin menyimpan. `policies`
// kosong-tapi-non-nil tetap menandai tenant TERKONFIGURASI (semua peran all-false)
// — beda dari default. `policies` nil menghapus konfigurasi (tenant kembali ke
// default terkunci).
func ReloadTenant(tenantID int64, policies map[string]Policy) {
	mu.Lock()
	defer mu.Unlock()
	if policies == nil {
		delete(byTenant, tenantID)
		return
	}
	byTenant[tenantID] = policies
}

// CanViewPhone: melihat nomor HP/WhatsApp UTUH. Terkonfigurasi → nilai baris (peran
// tanpa baris → false, fail-closed); belum → default: Sales & Admin.
func CanViewPhone(tenantID int64, role string) bool {
	mu.RLock()
	roles, configured := byTenant[tenantID]
	mu.RUnlock()
	if configured {
		return roles[role].CanViewPhone
	}
	return defaultView(role)
}

// CanEditPhone: menyunting nomor. Terkonfigurasi → nilai baris (fail-closed); belum
// → default: Sales saja. (Baris terkonfigurasi menegakkan edit⇒view di DB & handler.)
func CanEditPhone(tenantID int64, role string) bool {
	mu.RLock()
	roles, configured := byTenant[tenantID]
	mu.RUnlock()
	if configured {
		return roles[role].CanEditPhone
	}
	return defaultEdit(role)
}

// defaultView/defaultEdit = perilaku hardcode pra-BL-107, kini fallback saat tenant
// belum mengonfigurasi apa pun. Satu-satunya tempat default ini hidup. Allow-list
// (fail-closed): peran platform tersasar (super_admin/owner/staff BUKAN business_role)
// & nilai liar → tersembunyi, sumbu tetap tegak lurus.
func defaultView(role string) bool {
	return role == authz.BusinessRoleSales || role == authz.BusinessRoleAdmin
}

func defaultEdit(role string) bool {
	return role == authz.BusinessRoleSales
}

func group(rows []Row) map[int64]map[string]Policy {
	m := map[int64]map[string]Policy{}
	for _, r := range rows {
		byRole, ok := m[r.TenantID]
		if !ok {
			byRole = map[string]Policy{}
			m[r.TenantID] = byRole
		}
		byRole[r.BusinessRole] = Policy{CanViewPhone: r.CanViewPhone, CanEditPhone: r.CanEditPhone}
	}
	return m
}
