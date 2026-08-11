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
				{"crm:*", "write"}, {"crm:*", "approve"}, {"crm:roles", "write"},
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
				{"crm:quotes", "write"}, {"crm:quotes", "approve"},
				{"crm:sales_activity", "write"},
				{"crm:subscriptions", "write"}, {"crm:renewals", "write"},
				{"crm:plans", "read"}, {"crm:churn", "write"},
				{"crm:health", "write"}, {"crm:journey", "write"},
				{"crm:success_plans", "write"}, {"crm:adoption", "write"},
				{"crm:engagements", "write"},
				{"crm:renewal_mgmt", "write"}, {"crm:renewal_mgmt", "approve"},
				{"crm:playbooks", "write"}, {"crm:voc", "write"},
				{"crm:tickets", "write"}, {"crm:kb", "write"},
				{"crm:sla", "write"}, {"crm:activities", "write"},
				{"crm:reports", "read"},
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
				{"crm:quotes", "write"}, {"crm:sales_activity", "write"},
				{"crm:subscriptions", "read"}, {"crm:renewals", "read"},
				{"crm:plans", "read"}, {"crm:churn", "read"},
				{"crm:health", "read"}, {"crm:renewal_mgmt", "read"},
				{"crm:tickets", "read"}, {"crm:kb", "read"},
				{"crm:activities", "write"}, {"crm:reports", "read"},
			},
		},
		{
			Name: BusinessRoleCSM, DisplayName: "Customer Success Manager",
			Description: "Health score, renewal, onboarding desa binaan",
			DataScope:   DataScopeOwn, IsSystem: false,
			Perms: []DefaultPerm{
				{"crm:dashboard", "read"},
				{"crm:accounts", "write"}, {"crm:contacts", "write"},
				{"crm:deals", "read"}, {"crm:subscriptions", "read"},
				{"crm:renewals", "read"}, {"crm:plans", "read"},
				{"crm:churn", "write"}, {"crm:health", "write"},
				{"crm:journey", "write"}, {"crm:success_plans", "write"},
				{"crm:adoption", "write"}, {"crm:engagements", "write"},
				{"crm:renewal_mgmt", "write"}, {"crm:playbooks", "write"},
				{"crm:voc", "write"}, {"crm:tickets", "read"},
				{"crm:kb", "read"}, {"crm:sla", "read"},
				{"crm:activities", "write"}, {"crm:reports", "read"},
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
				{"crm:activities", "write"}, {"crm:reports", "read"},
			},
		},
	}
}

// ModuleDef = satu KOLOM matriks editor peran: objek Casbin + labelnya + apakah
// aksi "approve" bermakna baginya. Dipakai panel /roles (merender header matriks)
// DAN validasi RoleUpdate (menolak sel di luar daftar ini) — satu daftar, agar
// editor tak pernah menawarkan modul yang backend lalu tolak, dan sebaliknya.
type ModuleDef struct {
	Obj        string // objek Casbin, mis. "crm:accounts"
	Label      string // label layar (docs/crm/sistem-dan-role.md §4)
	CanApprove bool   // true → kolom "approve" aktif (hanya Deals/Quotes/Renewal Mgmt)
}

// crmModules = SELURUH modul CRM sebagai kolom matriks, urut sesuai nomor menu
// §4 (Dashboard … Reports). Settings (§9) & Customer Portal (§6.12) SENGAJA tak
// di sini: Settings ditegakkan sumbu tenant (canEditWorkspace), Portal di luar
// lingkup internal. "approve" hanya untuk tiga modul yang punya alur persetujuan
// (Deals, Quotes, Renewal Management) — sel approve modul lain tak bermakna dan
// tak dirender.
//
// crm:roles TIDAK di daftar ini: ia gerbang panel manajemen peran itu sendiri
// (dimiliki admin lewat glob crm:*), bukan modul yang matriksnya disunting orang
// — menampilkannya sebagai kolom akan mengundang admin mencabut aksesnya sendiri
// ke luar dari editor lewat pintu selain admin-lock.
var crmModules = []ModuleDef{
	{"crm:dashboard", "Dashboard", false},
	{"crm:accounts", "Accounts (Desa)", false},
	{"crm:contacts", "Contacts", false},
	{"crm:leads", "Leads", false},
	{"crm:deals", "Deals", true},
	{"crm:quotes", "Quotes", true},
	{"crm:sales_activity", "Sales Activity Log", false},
	{"crm:subscriptions", "Active Subscriptions", false},
	{"crm:renewals", "Renewals", false},
	{"crm:plans", "Plans & Pricing", false},
	{"crm:churn", "Churn / Cancellations", false},
	{"crm:health", "Health Score", false},
	{"crm:journey", "Journey / Onboarding", false},
	{"crm:success_plans", "Success Plans", false},
	{"crm:adoption", "Product Adoption", false},
	{"crm:engagements", "Engagements", false},
	{"crm:renewal_mgmt", "Renewal Management", true},
	{"crm:playbooks", "Playbooks", false},
	{"crm:voc", "Voice of Customer", false},
	{"crm:tickets", "Tickets / Cases", false},
	{"crm:kb", "Knowledge Base", false},
	{"crm:sla", "SLA Management", false},
	{"crm:activities", "Activities", false},
	{"crm:reports", "Reports", false},
}

// CRMModules mengembalikan salinan daftar modul (kolom matriks) agar pemanggil
// tak bisa memutasi slice paket. Urut = nomor menu §4.
func CRMModules() []ModuleDef {
	out := make([]ModuleDef, len(crmModules))
	copy(out, crmModules)
	return out
}

// ValidModuleObj melaporkan apakah obj adalah objek modul CRM yang boleh muncul
// di matriks editor — penjaga RoleUpdate agar hanya sel yang dikenal ditulis
// (input datang dari form user; objek liar tak boleh menyelinap jadi p-rule).
func ValidModuleObj(obj string) bool {
	for _, m := range crmModules {
		if m.Obj == obj {
			return true
		}
	}
	return false
}

// ModuleCanApprove melaporkan apakah objek modul mendukung aksi "approve" —
// penjaga agar sel approve tak ditulis untuk modul yang tak punya alur itu.
func ModuleCanApprove(obj string) bool {
	for _, m := range crmModules {
		if m.Obj == obj {
			return m.CanApprove
		}
	}
	return false
}

// ValidDataScope melaporkan apakah s adalah salah satu dari tiga tingkat cakupan
// F3 yang sah — penjaga RoleCreate/RoleUpdate (nilai datang dari dropdown user).
func ValidDataScope(s string) bool {
	return s == DataScopeAll || s == DataScopeOwn || s == DataScopeNone
}

// DefaultDataScope mengembalikan cakupan data F3 BAWAAN untuk sebuah nama peran.
// Ini SEKADAR nilai seed: setelah workspace dibuat, cakupan sesungguhnya hidup di
// kolom business_roles.data_scope dan dibaca per-request (RefreshIdentity) — jadi
// ini BUKAN sumber kebenaran runtime. Gunanya terbatas pada jalur yang belum/tak
// menyentuh DB (mis. harness test yang menyetel session dari nama peran).
//
// Nama tak dikenal → DataScopeNone (fail-closed, sejalan AccountsScopeFor): peran
// custom yang tak ada di daftar bawaan tak boleh diam-diam melihat semua desa.
func DefaultDataScope(name string) string {
	for _, r := range DefaultBusinessRoles() {
		if r.Name == name {
			return r.DataScope
		}
	}
	return DataScopeNone
}
