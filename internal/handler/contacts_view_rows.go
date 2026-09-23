package handler

import (
	"go_starter/internal/db"
)

// contacts_view_rows.go — bentuk antara contactListRow + converter dari kelima
// varian query daftar kontak global. Dipisah dari contacts_view.go krn ambang
// File Health (Route/Handler 150 baris).

// contactListRow = bentuk antara SATU baris daftar kontak global, sama untuk
// KELIMA varian query (default + 4 sort BL-157d). sqlc menghasilkan tipe Go
// TERPISAH per query bernama walau bentuk SELECT identik (db.ListContactsRow,
// db.ListContactsSortByCodeRow, dst) — contactListRow + converter di bawah
// menyatukannya kembali ke SATU pemetaan (mirror pola subListRow Subscriptions
// BL-157a) agar logika F4/masking tak terduplikasi 5×.
type contactListRow struct {
	ID                 int64
	AccountID          int64
	FirstName          string
	LastName           *string
	PositionCategory   *string
	ContactRole        *string
	MobilePhone        *string
	IsPrimaryContact   bool
	IsTechnicalContact bool
	EmailOptOut        bool
	DoNotContact       bool
	EntityCode         *string
	VillageName        string
}

func contactListRowFromDefault(r db.ListContactsRow) contactListRow {
	return contactListRow{
		ID: r.ID, AccountID: r.AccountID,
		FirstName: r.FirstName, LastName: r.LastName,
		PositionCategory: r.PositionCategory, ContactRole: r.ContactRole,
		MobilePhone:        r.MobilePhone,
		IsPrimaryContact:   r.IsPrimaryContact,
		IsTechnicalContact: r.IsTechnicalContact,
		EmailOptOut:        r.EmailOptOut, DoNotContact: r.DoNotContact,
		EntityCode: r.EntityCode, VillageName: r.VillageName,
	}
}

func contactListRowFromCodeSort(r db.ListContactsSortByCodeRow) contactListRow {
	return contactListRow{
		ID: r.ID, AccountID: r.AccountID,
		FirstName: r.FirstName, LastName: r.LastName,
		PositionCategory: r.PositionCategory, ContactRole: r.ContactRole,
		MobilePhone:        r.MobilePhone,
		IsPrimaryContact:   r.IsPrimaryContact,
		IsTechnicalContact: r.IsTechnicalContact,
		EmailOptOut:        r.EmailOptOut, DoNotContact: r.DoNotContact,
		EntityCode: r.EntityCode, VillageName: r.VillageName,
	}
}

func contactListRowFromNameSort(r db.ListContactsSortByNameRow) contactListRow {
	return contactListRow{
		ID: r.ID, AccountID: r.AccountID,
		FirstName: r.FirstName, LastName: r.LastName,
		PositionCategory: r.PositionCategory, ContactRole: r.ContactRole,
		MobilePhone:        r.MobilePhone,
		IsPrimaryContact:   r.IsPrimaryContact,
		IsTechnicalContact: r.IsTechnicalContact,
		EmailOptOut:        r.EmailOptOut, DoNotContact: r.DoNotContact,
		EntityCode: r.EntityCode, VillageName: r.VillageName,
	}
}

func contactListRowFromRoleSort(r db.ListContactsSortByRoleRow) contactListRow {
	return contactListRow{
		ID: r.ID, AccountID: r.AccountID,
		FirstName: r.FirstName, LastName: r.LastName,
		PositionCategory: r.PositionCategory, ContactRole: r.ContactRole,
		MobilePhone:        r.MobilePhone,
		IsPrimaryContact:   r.IsPrimaryContact,
		IsTechnicalContact: r.IsTechnicalContact,
		EmailOptOut:        r.EmailOptOut, DoNotContact: r.DoNotContact,
		EntityCode: r.EntityCode, VillageName: r.VillageName,
	}
}

func contactListRowFromVillageSort(r db.ListContactsSortByVillageRow) contactListRow {
	return contactListRow{
		ID: r.ID, AccountID: r.AccountID,
		FirstName: r.FirstName, LastName: r.LastName,
		PositionCategory: r.PositionCategory, ContactRole: r.ContactRole,
		MobilePhone:        r.MobilePhone,
		IsPrimaryContact:   r.IsPrimaryContact,
		IsTechnicalContact: r.IsTechnicalContact,
		EmailOptOut:        r.EmailOptOut, DoNotContact: r.DoNotContact,
		EntityCode: r.EntityCode, VillageName: r.VillageName,
	}
}
