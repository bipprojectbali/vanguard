package db

// ownership_sales.go — filter kepemilikan (data_scope) modul Sales: Leads,
// Deals, Subscriptions, Activities. Lihat ownership.go untuk inti & rasional.

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
