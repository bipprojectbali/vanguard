package db

// Implementation Tasks & Training Schedule (F3, via accounts) — CRM Modul 6,
// sub-item Onboarding 6.2.1.1/6.2.1.2. File terpisah dari ownership.go
// (sudah dekat batas 500 baris global CLAUDE.md rule 8) — bukan perubahan
// desain, murni pemecahan file demi file health.
//
// Kepemilikan mengikuti kolom accounts (account_owner / assigned_csm /
// backup_csm) — sama seperti CSRenewals & Engagements, tanpa kolom owner
// terpisah di tabel task/training itu sendiri (beda dari SuccessPlans).

// CSImplTasksListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListCSImplTasks. Paralel dengan CSRenewalsListFilter / EngagementsListFilter.
type CSImplTasksListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// CSImplTasksListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func CSImplTasksListFilterFor(dataScope string) CSImplTasksListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return CSImplTasksListFilter{ScopeAll: true}
	case ScopeOwn:
		return CSImplTasksListFilter{IsOwn: true}
	default: // ScopeNone
		return CSImplTasksListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT/MENGUBAH satu task —
// kembaran per-baris dari ListCSImplTasks. accountOwner/assignedCSM/backupCSM
// = kolom nullable dari accounts.
func (f CSImplTasksListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM *int64) bool {
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

// CSTrainingsListFilter = cakupan kepemilikan → dua flag boolean untuk
// ListCSTrainings. Identik strukturnya dengan CSImplTasksListFilter.
type CSTrainingsListFilter struct {
	ScopeAll bool
	IsOwn    bool
}

// CSTrainingsListFilterFor merakit flag untuk data_scope role. Fail-closed:
// nilai tak dikenal → ScopeNone (semua flag false → nol baris).
func CSTrainingsListFilterFor(dataScope string) CSTrainingsListFilter {
	switch AccountsScopeFor(dataScope) {
	case ScopeAll:
		return CSTrainingsListFilter{ScopeAll: true}
	case ScopeOwn:
		return CSTrainingsListFilter{IsOwn: true}
	default: // ScopeNone
		return CSTrainingsListFilter{}
	}
}

// Allows melaporkan apakah aktor (uid) boleh MELIHAT/MENGUBAH satu training —
// kembaran per-baris dari ListCSTrainings. accountOwner/assignedCSM/backupCSM
// = kolom nullable dari accounts.
func (f CSTrainingsListFilter) Allows(uid int64, accountOwner, assignedCSM, backupCSM *int64) bool {
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
