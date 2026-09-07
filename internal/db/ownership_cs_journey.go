package db

// ownership_cs_journey.go — F3 (ownership, via accounts) untuk Customer Journey
// / Lifecycle (CRM Modul 6, 6.2 — BL-77). File terpisah demi file health
// (ownership.go & ownership_cs_onboarding.go sudah padat) — bukan perubahan
// desain, murni pemecahan.
//
// Kepemilikan mengikuti kolom accounts (account_owner / assigned_csm /
// backup_csm) — SAMA seperti CSImplTasks/CSTrainings/CSRenewals, tanpa kolom
// owner terpisah di customer_success (satu baris ikut hidup account).

// CSJourneyListFilter = cakupan kepemilikan → dua flag boolean untuk query
// journey (KPI, phases, onboarding, accounts). Paralel CSImplTasksListFilter.
type CSJourneyListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// CSJourneyListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func CSJourneyListFilterFor(dataScope string) CSJourneyListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return CSJourneyListFilter{ScopeAll: true}
	case ScopeOwn:
		return CSJourneyListFilter{IsOwn: true}
	default: // ScopeNone
		return CSJourneyListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT satu baris journey desa —
// kembaran per-baris dari filter query. accountOwner/assignedCSM/backupCSM =
// kolom nullable dari accounts.
func (f CSJourneyListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM *int64) bool {
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
