package authz

// Role adalah tingkat otoritas sebagai ordinal — perbandingan hierarki O(1).
// Model 2-BIDANG (multi-tenancy), diurut sebagai satu tangga otoritas untuk
// perbandingan guard, tapi konseptual TEGAK LURUS:
//
//	TENANT   (scoped RLS):     member < admin < owner
//	PLATFORM (bypass RLS):     staff  < super_admin
//
// Platform > semua tenant (operator lintas-tenant). Nilai string map ke kolom
// users.role (member/admin/owner) ATAU role efektif session (staff/super_admin
// di-overlay RefreshIdentity — TAK disimpan di users.role).
type Role int8

const (
	RoleMember     Role = iota // tenant: anggota
	RoleAdmin                  // tenant: delegasi owner
	RoleOwner                  // tenant: pemilik (yang bayar)
	RoleStaff                  // platform: operator support (tabel platform_staff)
	RoleSuperAdmin             // platform: env-only (SUPER_ADMIN_EMAILS), immutable
)

// Nama role di DB / policy Casbin / role efektif session.
const (
	RoleNameMember     = "member"
	RoleNameAdmin      = "admin"
	RoleNameOwner      = "owner"
	RoleNameStaff      = "staff"
	RoleNameSuperAdmin = "super_admin"
)

// ParseRole memetakan string ke Role. Nilai tak dikenal → RoleMember (aman:
// otoritas terendah). Menerima role platform (staff/super_admin) dari session.
func ParseRole(s string) Role {
	switch s {
	case RoleNameSuperAdmin:
		return RoleSuperAdmin
	case RoleNameStaff:
		return RoleStaff
	case RoleNameOwner:
		return RoleOwner
	case RoleNameAdmin:
		return RoleAdmin
	default:
		return RoleMember
	}
}

// String mengembalikan nama role untuk DB/policy/session.
func (r Role) String() string {
	switch r {
	case RoleSuperAdmin:
		return RoleNameSuperAdmin
	case RoleStaff:
		return RoleNameStaff
	case RoleOwner:
		return RoleNameOwner
	case RoleAdmin:
		return RoleNameAdmin
	default:
		return RoleNameMember
	}
}

// ValidRoleName melaporkan apakah s adalah role TENANT yang boleh di-assign lewat
// panel (kolom users.role CHECK). Role platform TAK di sini: super_admin = env-only,
// staff = dikelola via platform_staff — bukan lewat set-role user biasa.
func ValidRoleName(s string) bool {
	return s == RoleNameMember || s == RoleNameAdmin || s == RoleNameOwner
}

// AssignableRoles mengembalikan role TENANT yang boleh dipilih di panel,
// terurut dari otoritas terendah. singleApp=true menghilangkan `owner`: mode
// single-app tak mengenal pemilik workspace (0006 §7) — aplikasinya dimiliki
// operator, dan puncak yang bisa DIANGKAT adalah admin.
//
// Satu sumber untuk semua dropdown role: kalau tiap view merakit daftarnya
// sendiri, salah satu pasti masih menawarkan owner di mode single.
func AssignableRoles(singleApp bool) []string {
	if singleApp {
		return []string{RoleNameMember, RoleNameAdmin}
	}
	return []string{RoleNameMember, RoleNameAdmin, RoleNameOwner}
}

// ValidBusinessRoleName melaporkan apakah s berformat sah sebagai NAMA peran CRM
// baru (sumbu bisnis, per-workspace). Ini validasi FORMAT, bukan keberadaan:
// apakah peran itu ADA di workspace divalidasi terpisah lewat GetBusinessRole.
//
// Nama menjadi subject Casbin (`t<id>:<name>`) DAN nilai memberships.business_role,
// jadi ia dibatasi ke [a-z0-9_], 2–32 char, diawali huruf: cukup untuk label
// mesin yang stabil, sempit agar tak ada spasi/kapital/tanda yang membuat "Sales"
// dan "sales" jadi dua peran berbeda yang membingungkan. Nama tampilan bebas ada
// di display_name; ini identitasnya.
func ValidBusinessRoleName(s string) bool {
	if len(s) < 2 || len(s) > 32 {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c == '_':
		case c >= '0' && c <= '9' && i > 0: // digit boleh, tapi tak di awal
		default:
			return false
		}
	}
	return s[0] >= 'a' && s[0] <= 'z'
}

// PlatformHomePath = rumah role PLATFORM (super_admin/staff). Panel lintas-
// workspace, jadi ia satu-satunya home yang tak bergantung workspace.
const PlatformHomePath = "/dev"

// IsPlatformHome melaporkan apakah role berumah di panel platform. Role TENANT
// (owner/admin/member) sengaja TIDAK punya home di sini: sejak keputusan 0004
// alamatnya "/w/{slug}" — bergantung WORKSPACE AKTIF, bukan role. Satu orang
// bisa owner di A dan member di B; memetakan role→path membuat alamat halaman
// yang sama berubah saat pindah workspace. Pembentukannya di handler.homeFor
// (butuh session slug, yang tak boleh diimpor paket authz).
func IsPlatformHome(role Role) bool {
	return role == RoleSuperAdmin || role == RoleStaff
}
