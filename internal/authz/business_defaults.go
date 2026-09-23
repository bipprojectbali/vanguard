package authz

// business_defaults.go — SATU sumber kebenaran untuk 5 peran CRM bawaan yang
// di-seed ke tiap workspace baru. Dulu kebenaran itu tersebar di
// business_policy.csv (di-load runtime) + CHECK constraint + konstanta Go; sejak
// 00007 peran hidup di DB dan bisa diedit per-workspace, jadi CSV berhenti jadi
// sumber runtime dan matriksnya pindah ke sini — dikonsumsi oleh:
//
//   - migrasi 00007 (backfill workspace yang sudah ada), dan
//   - seedBusinessRoles handler (workspace yang baru dibuat).
//
// Keduanya HARUS setara; test regresi mengunci matriks efektif = izin lama agar
// M2/M3 (Accounts/Contacts) tak diam-diam kehilangan akses.
//
// Ini definisi DEFAULT, bukan kebijakan runtime: setelah di-seed, operator
// workspace bebas menyuntingnya lewat panel /roles. Mengubah angka di sini hanya
// memengaruhi workspace yang dibuat SETELAHNYA.

// DefaultPerm = satu sel matriks (objek CRM × aksi). Cerminan satu p-rule Casbin
// dan satu baris business_role_permissions.
type DefaultPerm struct {
	Obj string
	Act string
}

// DefaultRole = satu peran bawaan lengkap: identitas, cakupan data F3, kunci
// sistem, dan matriks izinnya.
type DefaultRole struct {
	Name        string
	DisplayName string
	Description string // keterangan satu baris (kolom Deskripsi wireframe 9.2)
	DataScope   string // "all" | "own" | "none" — cakupan desa (F3)
	IsSystem    bool   // true hanya untuk admin (tak bisa diedit/dihapus)
	Perms       []DefaultPerm
}

// Nama data_scope — cerminan CHECK business_roles_scope_chk (00007). Dipakai
// seed, editor, dan pemetaan OwnershipScope (internal/db/ownership.go).
const (
	DataScopeAll  = "all"  // seluruh desa di workspace (Admin, Manager)
	DataScopeOwn  = "own"  // hanya desa yang ditugaskan (Sales, CSM)
	DataScopeNone = "none" // tak melihat desa lewat kepemilikan (Support, default)
)

// DefaultBusinessRoles mengembalikan 5 peran CRM bawaan + matriksnya, identik
// dengan backfill migrasi 00007. Dibangun sekali per panggilan (bukan var global)
// agar pemanggil tak bisa memutasi slice bersama.
//
// admin diwakili glob crm:* (write+approve) + crm:roles (write, panel manajemen
// peran) — menambah modul CRM baru tak menuntut baris admin baru. Peran lain
// dieja per-objek sesuai matriks docs/crm/sistem-dan-role.md §4.
func DefaultBusinessRoles() []DefaultRole {
	return []DefaultRole{
		{
			Name: BusinessRoleAdmin, DisplayName: "Administrator",
			Description: "Akses penuh seluruh modul & konfigurasi sistem",
			DataScope:   DataScopeAll, IsSystem: true,
			Perms: []DefaultPerm{
				{"crm:*", "write"}, {"crm:*", "approve"}, {"crm:*", "arr"},
				{"crm:roles", "write"},
			},
		},
		{
			Name: BusinessRoleManager, DisplayName: "Manager",
			Description: "Menyetujui diskon & renewal, lihat seluruh tim",
			DataScope:   DataScopeAll, IsSystem: false,
			Perms: []DefaultPerm{
				{"crm:dashboard", "read"},
				{"crm:accounts", "write"}, {"crm:contacts", "write"},
				{"crm:leads", "write"},
				{"crm:deals", "write"}, {"crm:deals", "approve"},
				{"crm:sales_activity", "write"},
				{"crm:subscriptions", "write"}, {"crm:subscriptions", "arr"},
				{"crm:renewals", "write"},
				{"crm:plans", "read"}, {"crm:churn", "write"},
				{"crm:health", "write"}, {"crm:journey", "write"},
				{"crm:success_plans", "write"}, {"crm:adoption", "write"},
				{"crm:engagements", "write"},
				{"crm:renewal_mgmt", "write"}, {"crm:renewals", "approve"},
				{"crm:playbooks", "write"},
				{"crm:tickets", "write"}, {"crm:kb", "write"},
				{"crm:sla", "write"}, {"crm:activities", "write"},
				{"crm:reports_sales", "read"}, {"crm:reports_cs", "read"},
				{"crm:reports_support", "read"}, {"crm:reports_subscriptions", "read"},
			},
		},
		{
			Name: BusinessRoleSales, DisplayName: "Sales",
			Description: "Leads, deals, quotes — hanya desa yang ditugaskan padanya",
			DataScope:   DataScopeOwn, IsSystem: false,
			Perms: []DefaultPerm{
				{"crm:dashboard", "read"},
				{"crm:accounts", "write"}, {"crm:contacts", "write"},
				{"crm:leads", "write"}, {"crm:deals", "write"},
				{"crm:sales_activity", "write"},
				{"crm:subscriptions", "read"}, {"crm:subscriptions", "arr"},
				{"crm:renewals", "read"},
				{"crm:plans", "read"}, {"crm:churn", "read"},
				{"crm:health", "read"}, {"crm:journey", "read"},
				{"crm:renewal_mgmt", "read"},
				{"crm:tickets", "read"}, {"crm:kb", "read"},
				{"crm:activities", "write"},
				{"crm:reports_sales", "read"}, {"crm:reports_cs", "read"},
				{"crm:reports_support", "read"}, {"crm:reports_subscriptions", "read"},
			},
		},
		{
			Name: BusinessRoleCSM, DisplayName: "Customer Success",
			Description: "Health score, renewal, onboarding desa binaan",
			DataScope:   DataScopeOwn, IsSystem: false,
			Perms: []DefaultPerm{
				{"crm:dashboard", "read"},
				{"crm:accounts", "write"}, {"crm:contacts", "write"},
				// BL-11: CS = peran pasca-jual; pipeline pra-jual (Deals) bukan
				// wilayahnya. Konteks kontrak TETAP lewat crm:subscriptions
				// (cermin pasca-jual dari deal menang: source_deal_id+nilai+plan).
				{"crm:subscriptions", "read"}, {"crm:subscriptions", "arr"},
				{"crm:renewals", "read"}, {"crm:plans", "read"},
				{"crm:churn", "write"}, {"crm:health", "write"},
				{"crm:journey", "write"}, {"crm:success_plans", "write"},
				{"crm:adoption", "write"}, {"crm:engagements", "write"},
				{"crm:renewal_mgmt", "write"}, {"crm:playbooks", "write"},
				{"crm:tickets", "read"},
				{"crm:kb", "read"}, {"crm:sla", "read"},
				{"crm:activities", "write"},
				{"crm:reports_sales", "read"}, {"crm:reports_cs", "read"},
				{"crm:reports_support", "read"}, {"crm:reports_subscriptions", "read"},
			},
		},
		{
			Name: BusinessRoleSupport, DisplayName: "Support",
			Description: "Tiket & SLA — tanpa akses data komersial",
			DataScope:   DataScopeNone, IsSystem: false,
			Perms: []DefaultPerm{
				{"crm:dashboard", "read"},
				{"crm:accounts", "read"}, {"crm:contacts", "read"},
				{"crm:health", "read"}, {"crm:tickets", "write"},
				{"crm:kb", "write"}, {"crm:sla", "read"},
				{"crm:activities", "write"},
				{"crm:reports_sales", "read"}, {"crm:reports_cs", "read"},
				{"crm:reports_support", "read"}, {"crm:reports_subscriptions", "read"},
			},
		},
	}
}
