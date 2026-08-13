package db

import "go_starter/internal/authz"

// ownership.go — isolasi ANTAR-DESA (inter-village) di layer aplikasi.
//
// RLS mengisolasi WORKSPACE (tenant_id); ia TIDAK memisahkan desa milik satu
// sales dari desa milik sales lain di workspace yang sama — keduanya baris
// accounts dengan tenant_id sama. Pemisahan itu ditegakkan DI SINI: satu tempat
// merakit klausa kepemilikan per CAKUPAN DATA (data_scope), dipakai SEMUA
// list-query modul.
//
// Kenapa BUKAN RLS lapis kedua (docs/crm/tasks.md §detail F3, sistem-dan-role §3):
// "atas data siapa" (Ownership Rule) tegak lurus terhadap "boleh apa"
// (Permission Set, sumbu Casbin F2). Ownership berubah tiap penugasan ulang desa
// — memodelkannya sebagai policy RLS berarti mengubah kebijakan DB tiap kali
// seorang CSM cuti. Ia data, bukan kebijakan; jadi filter query, bukan RLS.
//
// Cakupan DIBACA dari kolom business_roles.data_scope ('all'/'own'/'none'), BUKAN
// ditebak dari nama role. Peran kini bisa disunting per-workspace (F3 editable):
// sebuah role custom bernama "koordinator" boleh saja bercakupan 'own', dan
// menebak dari nama akan membuatnya jatuh ke ScopeNone secara senyap. Sumbernya
// satu — kolom di DB, di-refresh per-request ke session (SetBusinessDataScope).

// OwnershipScope = seberapa luas sebuah role boleh melihat desa DALAM
// workspace-nya. Bukan tentang izin verb (itu Casbin), melainkan cakupan baris.
type OwnershipScope int

const (
	// ScopeNone — tak melihat desa apa pun lewat kepemilikan. Default DEFENSIF:
	// data_scope kosong / tak dikenal / 'none' (mis. Support, yang menyentuh desa
	// HANYA lewat konteks tiket, bukan daftar desa umum) jatuh ke sini. Klausa
	// "FALSE" dipilih ketimbang mengembalikan galat: list-query tetap jalan & aman
	// mengembalikan nol baris, alih-alih meledak di jalur yang jarang dilewati.
	ScopeNone OwnershipScope = iota
	// ScopeOwn — hanya desa yang ia miliki/bina. Cakupan ◐ di matriks §4. Untuk
	// role custom, "miliki" = UNION kolom kepemilikan (account_owner ATAU
	// assigned_csm ATAU backup_csm) — satu makna "desa yang ditugaskan padaku"
	// tanpa perlu tahu peran spesifik apa yang menautkannya.
	ScopeOwn
	// ScopeAll — seluruh desa di workspace (data_scope 'all'). Cakupan ✓. Tetap
	// tunduk Field-Level Security (F4) — melihat luas ≠ melihat segalanya.
	ScopeAll
)

// AccountsScopeFor memetakan data_scope role → cakupan baris pada hub accounts.
// Nilai tak dikenal / "" → ScopeNone (aman: nol baris, bukan bocor). Aktor
// platform / tanpa role CRM punya data_scope "" → ScopeNone, tak melihat desa.
func AccountsScopeFor(dataScope string) OwnershipScope {
	switch dataScope {
	case authz.DataScopeAll:
		return ScopeAll
	case authz.DataScopeOwn:
		return ScopeOwn
	default: // authz.DataScopeNone, "", nilai liar
		return ScopeNone
	}
}

// AccountsOwnershipClause merakit fragmen WHERE kepemilikan untuk di-AND-kan ke
// list-query atas accounts. startIdx = nomor placeholder $N bebas berikutnya
// (query bisa sudah memakai $1=tenant_id, dst).
//
// Kembalian:
//   - clause: ekspresi boolean TANPA "AND" di depan (mis. "account_owner = $2").
//     "" untuk ScopeAll (tanpa batas). "FALSE" untuk ScopeNone (nol baris).
//   - args: nilai untuk placeholder di clause (uid, sekali — di-refer ulang bila
//     perlu, jadi klausa yang menyebut $N tiga kali tetap satu arg).
//   - nextIdx: $N bebas berikutnya, agar pemanggil bisa menyambung placeholder.
//
// ScopeOwn ⇒ UNION kepemilikan: account_owner ATAU assigned_csm ATAU backup_csm =
// uid. Satu klausa untuk semua role bercakupan 'own' — seeded (sales cuma jadi
// account_owner, CSM cuma assigned/backup) maupun custom. Union adalah superset
// yang aman: seorang sales murni tak pernah muncul di kolom CSM, jadi hasilnya
// identik dengan klausa spesifik lama, tanpa perlu bercabang per nama role.
// Catatan: "Sales asal" (baca desa yang dulu ia tutup, §8.2) menunggu kolom
// sales_owner di konversi deal (M4) — belum ada di 00005, ditinggalkan eksplisit.
func AccountsOwnershipClause(dataScope string, uid int64, startIdx int) (clause string, args []any, nextIdx int) {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return "", nil, startIdx
	case ScopeOwn:
		// Satu placeholder di-refer tiga kali → satu arg, bukan tiga.
		p := placeholder(startIdx)
		return "(account_owner = " + p + " OR assigned_csm = " + p + " OR backup_csm = " + p + ")",
			[]any{uid}, startIdx + 1
	default: // ScopeNone
		return "FALSE", nil, startIdx
	}
}

// AccountsListFilter = cakupan kepemilikan diterjemahkan ke flag boolean untuk
// ListAccounts (sqlc murni). Diturunkan dari AccountsScopeFor — sumber yang SAMA
// dengan AccountsOwnershipClause — supaya "siapa lihat apa" tak pernah bercabang
// jadi dua kebenaran antara list-query dan count/probe.
//
//   - ScopeAll  → ScopeAll=true         (lihat semua di workspace)
//   - ScopeOwn  → IsOwn=true            (union: owner/assigned_csm/backup_csm = uid)
//   - ScopeNone → keduanya false → query mengembalikan NOL baris (fail-closed)
//
// ListAccounts/ListContacts di-generate dengan DUA flag SQL (is_sales/is_csm)
// yang di-OR-kan; IsOwn dipetakan ke KEDUANYA true di call site (union kolom
// kepemilikan). Flag SQL itu detail implementasi query — konsepnya di sini
// tunggal: "punyaku atau bukan".
type AccountsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// AccountsListFilterFor merakit flag untuk data_scope role. Fail-closed: nilai
// tak dikenal jatuh ke ScopeNone (semua flag false → nol baris), sama dengan
// klausa "FALSE" pada AccountsOwnershipClause.
func AccountsListFilterFor(dataScope string) AccountsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return AccountsListFilter{ScopeAll: true}
	case ScopeOwn:
		return AccountsListFilter{IsOwn: true}
	default: // ScopeNone
		return AccountsListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT satu baris account dengan
// kolom kepemilikan tertentu. Ini kembaran per-baris dari ListAccounts: daftar
// menyaring lewat SQL, detail (GetAccount, yang sengaja tak ber-ownership)
// memutuskan lewat SINI — keduanya diturunkan dari flag yang sama, jadi "boleh
// lihat di daftar" dan "boleh buka detail" tak pernah berbeda jawabannya.
//
// owner/assignedCSM/backupCSM = kolom nullable dari baris (nil = tak diisi).
// Dipakai handler untuk memilih 404 (bukan 403) atas desa di luar cakupan:
// menyangkal keberadaannya, bukan mengakuinya lalu menolak.
func (f AccountsListFilter) Allows(uid int64, owner, assignedCSM, backupCSM *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn {
		if owner != nil && *owner == uid {
			return true
		}
		if assignedCSM != nil && *assignedCSM == uid {
			return true
		}
		if backupCSM != nil && *backupCSM == uid {
			return true
		}
	}
	return false
}

// ── Leads & Deals (F3, satu kolom kepemilikan) ─────────────────────────────
//
// Beda dari accounts (tiga kolom: owner/assigned_csm/backup_csm), lead & deal
// punya SATU kolom kepemilikan (lead_owner / deal_owner) — skema §4: sales yang
// menutup deal boleh berbeda dari owner desa, jadi kepemilikannya berdiri sendiri.
// Cakupan tetap diturunkan dari AccountsScopeFor (sumber SATU untuk data_scope →
// ScopeAll/Own/None), hanya predikat baris yang lebih ringkas.

// LeadsListFilter = cakupan kepemilikan → dua flag boolean untuk ListLeads.
// ScopeAll → lihat semua; IsOwn → lead_owner = uid; keduanya false → NOL baris
// (fail-closed). Diturunkan dari AccountsScopeFor agar "siapa lihat apa" tak
// bercabang jadi dua kebenaran.
type LeadsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// LeadsListFilterFor merakit flag untuk data_scope role. Fail-closed: nilai tak
// dikenal → ScopeNone (semua flag false → nol baris).
func LeadsListFilterFor(dataScope string) LeadsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return LeadsListFilter{ScopeAll: true}
	case ScopeOwn:
		return LeadsListFilter{IsOwn: true}
	default: // ScopeNone
		return LeadsListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT satu lead dengan owner
// tertentu — kembaran per-baris dari ListLeads. Dipakai handler untuk memilih 404
// (bukan 403) atas lead di luar cakupan. owner = kolom nullable (nil = tak diisi).
func (f LeadsListFilter) Allows(uid int64, owner *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn && owner != nil && *owner == uid {
		return true
	}
	return false
}

// DealsListFilter = cakupan kepemilikan → dua flag boolean untuk ListDeals.
// Bentuk identik LeadsListFilter (satu kolom deal_owner); tipe terpisah agar
// call-site jelas modul mana yang disaring dan tak tertukar argumen.
type DealsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// DealsListFilterFor merakit flag untuk data_scope role. Fail-closed sama.
func DealsListFilterFor(dataScope string) DealsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return DealsListFilter{ScopeAll: true}
	case ScopeOwn:
		return DealsListFilter{IsOwn: true}
	default: // ScopeNone
		return DealsListFilter{}
	}
}

// Allows — kembaran per-baris dari ListDeals (deal_owner). 404-gate detail deal.
func (f DealsListFilter) Allows(uid int64, owner *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn && owner != nil && *owner == uid {
		return true
	}
	return false
}

// SubscriptionsListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListSubscriptions. Langganan membawa kolom subscription_owner — sumbu data_scope
// yang SAMA dengan deals (satu kolom pemilik), jadi bentuknya identik DealsListFilter;
// tipe terpisah agar call-site jelas modul mana yang disaring dan tak tertukar argumen.
type SubscriptionsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// SubscriptionsListFilterFor merakit flag untuk data_scope role. Fail-closed sama:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func SubscriptionsListFilterFor(dataScope string) SubscriptionsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return SubscriptionsListFilter{ScopeAll: true}
	case ScopeOwn:
		return SubscriptionsListFilter{IsOwn: true}
	default: // ScopeNone
		return SubscriptionsListFilter{}
	}
}

// Allows — kembaran per-baris dari ListSubscriptions (subscription_owner). 404-gate
// detail langganan. owner = kolom nullable (nil = tak diisi).
func (f SubscriptionsListFilter) Allows(uid int64, owner *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn && owner != nil && *owner == uid {
		return true
	}
	return false
}

// ActivitiesListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListActivities. Bentuk identik LeadsListFilter/DealsListFilter (satu kolom
// owner_id); tipe terpisah agar call-site jelas modul mana yang disaring dan tak
// tertukar argumen. Sales Activity Log (4.4) memakainya di atas baris
// activity_context='sales'.
type ActivitiesListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// ActivitiesListFilterFor merakit flag untuk data_scope role. Fail-closed sama:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func ActivitiesListFilterFor(dataScope string) ActivitiesListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return ActivitiesListFilter{ScopeAll: true}
	case ScopeOwn:
		return ActivitiesListFilter{IsOwn: true}
	default: // ScopeNone
		return ActivitiesListFilter{}
	}
}

// Allows — kembaran per-baris dari ListActivities (owner_id). 404-gate detail
// aktivitas. owner = kolom nullable (nil = tak diisi).
func (f ActivitiesListFilter) Allows(uid int64, owner *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn && owner != nil && *owner == uid {
		return true
	}
	return false
}

// ── Tickets (F3, dengan override Support) ──────────────────────────────────
//
// Tiket berbeda dari entitas lain: Support punya data_scope='none' (di daftar
// desa mereka lihat 0 baris) tapi HARUS lihat semua tiket workspace (wireframe
// 6.9: "Support menangani"). Override dikodekan di sini agar satu sumber
// kebenaran — tak ada cabang logika tersembunyi di handler.

// TicketsListFilter = cakupan kepemilikan → dua flag boolean untuk ListTickets.
// Paralel dengan AccountsListFilter, tapi punya jalur override Support.
type TicketsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// TicketsListFilterFor merakit flag untuk data_scope role + override Support.
// canWrite = canWriteTickets(ctx): support (data_scope='none' + write) → ScopeAll.
// Fail-closed: nilai tak dikenal & !canWrite → semua flag false → nol baris.
func TicketsListFilterFor(dataScope string, canWrite bool) TicketsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return TicketsListFilter{ScopeAll: true}
	case ScopeOwn:
		return TicketsListFilter{IsOwn: true}
	default: // ScopeNone (Support: data_scope='none' tapi lihat semua tiket)
		if canWrite {
			return TicketsListFilter{ScopeAll: true}
		}
		return TicketsListFilter{} // fail-closed: nol baris
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT satu tiket — kembaran
// per-baris dari ListTickets. Dipakai UpdateTicketStatus untuk memilih 404
// (menyangkal keberadaan) atas tiket di luar cakupan.
// accountOwner/assignedCSM/backupCSM = kolom nullable dari accounts (nil = tak diisi).
func (f TicketsListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn {
		if accountOwner != nil && *accountOwner == uid {
			return true
		}
		if assignedCSM != nil && *assignedCSM == uid {
			return true
		}
		if backupCSM != nil && *backupCSM == uid {
			return true
		}
	}
	return false
}

// placeholder membentuk "$N" untuk pgx. Dipisah agar niatnya terbaca dan mudah
// diuji; strconv sengaja dihindari untuk N kecil yang sangat sering dipanggil.
func placeholder(n int) string {
	return "$" + itoa(n)
}

// itoa — konversi int→desimal minimal (N selalu ≥ 1 di sini). Menghindari
// import strconv untuk satu pemakaian panas; N tak pernah negatif.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
