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

// ModuleDef = satu KOLOM matriks editor peran: objek Casbin + labelnya + apakah
// aksi "approve" bermakna baginya. Dipakai panel /roles (merender header matriks)
// DAN validasi RoleUpdate (menolak sel di luar daftar ini) — satu daftar, agar
// editor tak pernah menawarkan modul yang backend lalu tolak, dan sebaliknya.
type ModuleDef struct {
	Obj        string // objek Casbin, mis. "crm:accounts"
	Label      string // label layar (docs/crm/sistem-dan-role.md §4)
	CanApprove bool   // true → kolom "approve" aktif (hanya Deals/Renewals)
	CanARR     bool   // true → kolom "Lihat ARR" aktif (hanya Subscriptions, BL-58)
	// WriteEnforced: true bila ADA titik enforcement nyata yang membedakan
	// "write" dari "read" untuk modul ini (mis. CanBusiness(ctx,obj,"write")
	// di suatu handler). false → modul HANYA pernah dicek "read" di mana pun
	// (audit 2026-09: dashboard, subscriptions, activities, 4 halaman
	// Reports) — opsi "Kelola" disembunyikan di editor (role_edit_levels.go)
	// karena menyetel "write" tak memberi kemampuan tambahan apa pun di atas
	// "read", hanya menyesatkan admin yang mengira mencentangnya membuka
	// aksi tulis. business.conf: write MENCAKUP read, jadi baris write LAMA
	// (mis. Manager crm:subscriptions write) tetap berfungsi sbg read — tak
	// ada regresi akses, hanya UI-nya yang tak lagi menawarkan "Kelola".
	WriteEnforced bool
}

// crmModules = SELURUH modul CRM sebagai kolom matriks, urut sesuai nomor menu
// §4 (Dashboard … Reports). Settings (§9) & Customer Portal (§6.12) SENGAJA tak
// di sini: Settings ditegakkan sumbu tenant (canEditWorkspace), Portal di luar
// lingkup internal. "approve" hanya untuk dua modul yang punya alur persetujuan
// (Deals, Renewals) — sel approve modul lain tak bermakna dan tak dirender.
//
// crm:roles TIDAK di daftar ini: ia gerbang panel manajemen peran itu sendiri
// (dimiliki admin lewat glob crm:*), bukan modul yang matriksnya disunting orang
// — menampilkannya sebagai kolom akan mengundang admin mencabut aksesnya sendiri
// ke luar dari editor lewat pintu selain admin-lock.
//
// crm:quotes (BL-56) & crm:voc (BL-57) SENGAJA tak di daftar ini walau objeknya
// ada di enforcer. Quotes: akses quote MEWARISI crm:deals (nest di bawah deal,
// tak ada satu pun CanBusiness(ctx,"crm:quotes",…)) → kolomnya inert; dibuang
// agar editor tak menawarkan toggle tanpa efek. VoC: modulnya belum dibangun
// (nol route/handler/menu) → dikembalikan ke daftar ini saat modul VoC dibangun.
// Kolom (Obj, Label, CanApprove, CanARR). CanApprove hanya Renewals (BL-145
// subtask 2, dipindahkan dari Renewal Mgmt): tombol Setujui/Reject renewal
// upsell HANYA muncul di halaman Subscription detail (dunia Renewals), NOL
// kemunculan di halaman Renewal Management manapun (verifikasi UI nyata, bukan
// cuma nama objek Casbin) — kolom "Setujui" di baris Renewal Mgmt dulu
// menyesatkan admin yang mengira mencentangnya mengizinkan approve DI HALAMAN
// itu. crm:renewal_mgmt TETAP di daftar ini utk read/write (gate halaman
// Renewal Management CS sendiri, cs_renewals_view.go) — hanya act approve yang
// pindah rumah objek. canApproveRenewal (subscriptions_view.go) & default grant
// Manager ikut disesuaikan; migrasi 00048 memindah baris existing.
//
// Deals SENGAJA CanApprove=false juga (BL-145 subtask 1) walau objek
// crm:deals/approve TETAP ada di enforcer & default grant Manager TETAP
// dipertahankan (canApproveDeals, sales_view.go, sudah diekspos sbg sumber
// tunggal utk alur approval Deal yg menyusul) — kolom di editor DISEMBUNYIKAN
// krn nol enforcement point hari ini (belum dipakai di slice manapun), jadi
// checkbox aktif akan menyesatkan admin yg mengira sudah ada alur approval
// Deal. Pola SAMA dgn crm:quotes/crm:voc di bawah: readRoleMatrix
// full-replace per role (bukan patch), jadi kapabilitas yg tak terwakili di
// editor (di luar daftar, ATAU CanApprove/CanARR=false) ikut TERHAPUS begitu
// role itu disunting ULANG lewat UI — bukan permanen aman, tapi konsisten dgn
// invarian existing (TestRoles_UpdateIgnoresDroppedColumns), bukan risiko
// baru. CanARR checkbox-nya cuma dirender di baris Subscriptions (BL-58) —
// TAPI sejak BL-169 kapabilitas yang sama (crm:subscriptions/arr) juga
// menggerbangi Nilai Kontrak lintas modul (Deal Amount, estimasi Lead,
// anggaran desa Account, dst — canSeeARR, internal/handler/fls.go), bukan
// cuma ARR Subscriptions. Checkbox tetap satu, TAK dipindah/diduplikasi ke
// modul lain — menghindari kolom "Lihat ARR" ganda yang membingungkan. Syarat
// aktif/simpannya sendiri diperluas lintas modul lewat ARRGateObjects (lihat
// di bawah) — sebelumnya cuma bisa dicentang bila Subscriptions sendiri
// granted, walau field yang sama sudah tampil di modul lain.
// Kolom ke-5 (WriteEnforced): false pada 7 modul yang diaudit 2026-09 TANPA
// satu pun CanBusiness(ctx,obj,"write") nyata (hanya "read" di mana pun) —
// dashboard, subscriptions, activities, & 4 halaman Reports. Sisanya true.
//
// crm:renewals & crm:churn (audit 2026-09, KASUS BEDA dari WriteEnforced=false
// di atas — di sini write-nya NYATA ditegakkan, tapi tak terjangkau UI tanpa
// modul lain): keduanya berdiri sendiri di matriks ini, TAPI tiga jalur yang
// membawa ke halaman/tombolnya — SubscriptionDetail (tempat tombol Renew/Churn
// dirender), dasbor Renewals, dasbor Churn (subscriptions_detail.go,
// subscriptions_renewals.go, subscriptions_churn_page.go) — semua digerbangi
// canViewSubscriptions ("crm:subscriptions" read), BUKAN crm:renewals/crm:churn.
// Jadi peran dgn Active Subscriptions="Tak ada" + Renewals="Kelola" akan 403 di
// ketiga halaman itu; grant crm:renewals write TETAP tersimpan sah & AKTIF di
// endpoint POST (canRenewSubscriptions/canChurnSubscriptions, subscriptions_
// renew.go/churn.go, tak ikut mengecek crm:subscriptions), hanya saja tak ada
// jalur UI utk memicunya tanpa Active Subscriptions ≥ Lihat juga. Diputuskan
// didokumentasikan (bukan diubah gate-nya) — lihat moduleHints di role_edit.go
// (peringatan kecil di editor) & canViewRenewals (subscriptions_view.go, gerbang
// READ Renewals yg simetris tapi tanpa pemanggil krn alasan yang sama).
var crmModules = []ModuleDef{
	{"crm:dashboard", "Dashboard", false, false, false},
	{"crm:accounts", "Accounts (Desa)", false, false, true},
	{"crm:contacts", "Contacts", false, false, true},
	{"crm:leads", "Leads", false, false, true},
	{"crm:deals", "Deals dan Quotes", false, false, true},
	{"crm:sales_activity", "Sales Activity Log", false, false, true},
	{"crm:subscriptions", "Active Subscriptions", false, true, false},
	{"crm:renewals", "Renewals", true, false, true},
	{"crm:plans", "Plans & Pricing", false, false, true},
	{"crm:churn", "Churn / Cancellations", false, false, true},
	{"crm:health", "Health Score", false, false, true},
	{"crm:journey", "Journey / Onboarding", false, false, true},
	{"crm:success_plans", "Success Plans", false, false, true},
	// BL-169: crm:adoption dulu "Product Adoption" (gerbang section CS 360
	// yang kini numpang crm:journey) — dipakai ulang sbg gerbang khusus
	// "Training Schedule" (modul nyata, dulu numpang crm:journey juga).
	// Objek Casbin & posisi TETAP; hanya label yang berubah.
	{"crm:adoption", "Training Schedule", false, false, true},
	{"crm:engagements", "Engagements", false, false, true},
	{"crm:renewal_mgmt", "Renewal Management", false, false, true},
	{"crm:playbooks", "Playbooks", false, false, true},
	{"crm:tickets", "Tickets / Cases", false, false, true},
	{"crm:kb", "Knowledge Base", false, false, true},
	{"crm:sla", "SLA Management", false, false, true},
	{"crm:activities", "Activities", false, false, false},
	// BL-169: crm:reports (1 objek, gerbang preset Sales/Subscription report)
	// dipecah jadi 4 objek sesuai 4 halaman Reports nyata di sidebar — admin
	// bisa memberi akses per-domain (mis. Sales lihat Sales Report saja).
	// Dirender FLAT (tanpa header grup), sama seperti modul lain.
	{"crm:reports_sales", "Sales Reports", false, false, false},
	{"crm:reports_cs", "Customer Success Reports", false, false, false},
	{"crm:reports_support", "Support Reports", false, false, false},
	{"crm:reports_subscriptions", "Subscription Reports", false, false, false},
}

// CRMModules mengembalikan salinan daftar modul (kolom matriks) agar pemanggil
// tak bisa memutasi slice paket. Urut = nomor menu §4.
func CRMModules() []ModuleDef {
	out := make([]ModuleDef, len(crmModules))
	copy(out, crmModules)
	return out
}

// ARRGateObjects = modul yang levelnya (read/write) memenuhi syarat mengaktifkan
// & menyimpan (crm:subscriptions, arr) — checkbox "Lihat Nilai Kontrak" TETAP
// SATU (dirender hanya di baris Subscriptions, CanARR di atas, hindari kolom
// ganda yg membingungkan), tapi kapabilitasnya sendiri sudah lintas modul sejak
// BL-169 (canSeeARR, internal/handler/fls.go, menggerbangi Deal Amount, estimasi
// Lead, anggaran desa Account, Quote, Renewals & Churn (nilai kontrak yg akan
// habis/hilang), & 3 dari 4 Reports). Sebelum daftar ini, checkbox itu hanya
// bisa dicentang/tersimpan bila Subscriptions sendiri punya akses baca/tulis —
// role yang cuma diberi mis. Leads="Kelola" tak pernah bisa mengaktifkannya
// walau field yang sama sudah kelihatan di halaman Leads. Dipakai DUA sisi:
// backend (readRoleMatrix, roles_rest.go — guard simpan) & frontend
// (role_edit.go — ekspresi disabled reaktif checkbox), jadi keduanya tak bisa
// menyimpang. crm:dashboard SENGAJA TAK diikutkan walau widget MRR di sana
// juga memanggil canSeeARR: crm:dashboard/read ada di SEMUA peran bawaan
// (lihat grant di atas) — memasukkannya bikin gate ini nyaris selalu terbuka,
// meniadakan gunanya. Reports_support SENGAJA tak diikutkan juga — halaman itu
// tak punya field finance apa pun (nol enforcement point utk canSeeARR di
// sana), jadi menyertakannya cuma memperluas gate tanpa efek nyata.
var ARRGateObjects = []string{
	"crm:subscriptions",
	"crm:deals",
	"crm:leads",
	"crm:accounts",
	"crm:renewals",
	"crm:churn",
	"crm:reports_sales",
	"crm:reports_cs",
	"crm:reports_subscriptions",
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

// ModuleCanARR melaporkan apakah objek modul mendukung aksi "arr" (visibilitas
// ARR, BL-58) — penjaga agar sel arr tak ditulis untuk modul selain Subscriptions.
func ModuleCanARR(obj string) bool {
	for _, m := range crmModules {
		if m.Obj == obj {
			return m.CanARR
		}
	}
	return false
}

// ModuleWriteEnforced melaporkan apakah modul punya titik enforcement "write"
// nyata — penjaga agar editor (role_edit_levels.go) hanya menawarkan opsi
// "Kelola" untuk modul yang benar-benar membedakannya dari "read", dan agar
// readRoleMatrix (roles_rest.go) menurunkan sel "write" yang nyasar masuk
// jadi "read" (defense-in-depth; form seharusnya tak pernah mengirim "write"
// untuk modul ini krn opsinya tak dirender). Objek tak dikenal → false
// (fail-closed, sejalan ModuleCanApprove/ModuleCanARR).
func ModuleWriteEnforced(obj string) bool {
	for _, m := range crmModules {
		if m.Obj == obj {
			return m.WriteEnforced
		}
	}
	return false
}

// ModuleARRGate melaporkan apakah objek modul ada di ARRGateObjects — dipakai
// roleModuleRows (roles_card.go) menandai RoleModulePerm.ARREligible tiap baris,
// dikonsumsi view role_edit.go utk ekspresi disabled reaktif checkbox ARR.
func ModuleARRGate(obj string) bool {
	for _, o := range ARRGateObjects {
		if o == obj {
			return true
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
