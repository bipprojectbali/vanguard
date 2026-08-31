package db

// ownership_cs.go — filter kepemilikan (data_scope) modul Customer Success:
// Tickets (override Support), Engagements, CSRenewals, SuccessPlans.
// Lihat ownership.go untuk inti & rasional.

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

// ── Engagements (F3, via accounts) ───────────────────────────────────────────
//
// Engagement dikaitkan ke desa (account_id); kepemilikan mengikuti kolom
// accounts (account_owner / assigned_csm / backup_csm). Bentuk identik dengan
// AccountsListFilter — "desa yang aku bina" → "engagement untuk desa itu".
// Admin/Manager (ScopeAll) lihat semua; CSM (ScopeOwn) lihat engagement desa
// binarannya. Sales tidak punya crm:engagements di policy → fail F2 sebelum
// sampai ke filter F3 ini. Support juga tidak punya → sama.

// EngagementsListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListEngagements. Paralel dengan AccountsListFilter.
type EngagementsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// EngagementsListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func EngagementsListFilterFor(dataScope string) EngagementsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return EngagementsListFilter{ScopeAll: true}
	case ScopeOwn:
		return EngagementsListFilter{IsOwn: true}
	default: // ScopeNone
		return EngagementsListFilter{}
	}
}

// CSRenewalsListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListCSRenewals. Ownership via AKUN (assigned_csm/backup_csm/account_owner),
// bukan subscription_owner — CSM harus melihat renewal desa binaannya terlepas
// siapa sales-owner langganannya. Identik dengan TicketsListFilter (minus
// SupportOverride) dan EngagementsListFilter.
type CSRenewalsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// CSRenewalsListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func CSRenewalsListFilterFor(dataScope string) CSRenewalsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return CSRenewalsListFilter{ScopeAll: true}
	case ScopeOwn:
		return CSRenewalsListFilter{IsOwn: true}
	default: // ScopeNone
		return CSRenewalsListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT satu baris renewal —
// kembaran per-baris dari ListCSRenewals. 404-gate detail renewal.
// accountOwner/assignedCSM/backupCSM = kolom nullable dari accounts.
func (f CSRenewalsListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM *int64) bool {
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

// ── Success Plans (F3, via accounts) ────────────────────────────────────────
//
// Success Plan dikaitkan ke desa (account_id); kepemilikan mengikuti kolom
// accounts (account_owner / assigned_csm / backup_csm) PLUS kolom owner_csm
// pada plan itu sendiri — CSM pemilik plan tetap bisa mengelolanya walau desa
// berpindah tangan. Admin/Manager (ScopeAll) lihat semua. Sales tidak punya
// crm:success_plans di policy → fail F2 sebelum sampai ke filter F3 ini.

// SuccessPlansListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListSuccessPlans. Paralel dengan EngagementsListFilter / CSRenewalsListFilter.
type SuccessPlansListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// SuccessPlansListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func SuccessPlansListFilterFor(dataScope string) SuccessPlansListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return SuccessPlansListFilter{ScopeAll: true}
	case ScopeOwn:
		return SuccessPlansListFilter{IsOwn: true}
	default: // ScopeNone
		return SuccessPlansListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT/MENGUBAH satu success plan —
// kembaran per-baris dari ListSuccessPlans. Memeriksa kepemilikan AKUN
// (account_owner / assigned_csm / backup_csm) SERTA owner_csm plan sendiri,
// sehingga CSM yang membuat plan tetap bisa mengelolanya walau desa dipindahkan.
func (f SuccessPlansListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM, ownerCSM *int64) bool {
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
		if ownerCSM != nil && *ownerCSM == uid {
			return true
		}
	}
	return false
}
