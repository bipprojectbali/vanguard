package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// contacts_view.go — pemetaan model DB → data siap-render view (murni-data).
// Penyamaran Field-Level Security (F4) dilakukan DI SINI, di handler, sebelum
// nilai menyentuh view: yang tak berhak TAK PERNAH menerima nomor aslinya (lihat
// fls.go). Kontak punya nomor pribadi HP/WhatsApp (satu kolom mobile_phone; kolom
// whatsapp_number lama tetap ada di DB tapi tak lagi ditampilkan) — PII, disamarkan
// bagi non-Sales; telepon kantor = nomor kelembagaan, tak disamarkan (sejajar
// OfficePhone di accounts).

// contactRowView memetakan satu baris daftar kontak SATU desa. Nomor HP/WhatsApp
// ikut di baris (wireframe M3) tapi DISAMARKAN di sini (F4) bila aktor bukan Sales —
// nomor asli tak pernah dioper ke view. Village kosong (daftar per-desa tak berkolom
// Desa). LastActivity ditunda modul Activities → "" (view: "—").
func contactRowView(ctx context.Context, c db.Contact) panel.ContactRow {
	return panel.ContactRow{
		ID:               c.ID,
		AccountID:        c.AccountID,
		EntityCode:       deref(c.EntityCode),
		Name:             contactFullName(c),
		PositionCategory: deref(c.PositionCategory),
		ContactRole:      deref(c.ContactRole),
		Phone:            maskPhone(ctx, deref(c.MobilePhone)),
		IsPrimary:        c.IsPrimaryContact,
		IsTechnical:      c.IsTechnicalContact,
		EmailOptOut:      c.EmailOptOut,
		DoNotContact:     c.DoNotContact,
	}
}

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

// contactRowViewGlobal memetakan satu baris daftar kontak LINTAS-desa. Sama dengan
// contactRowView + kolom Desa (village_name dari JOIN accounts). Menerima
// contactListRow (bentuk antara, lihat komentar di atas) alih-alih tipe row sqlc
// langsung — satu fungsi ini melayani kelima varian query (default + 4 sort).
func contactRowViewGlobal(ctx context.Context, r contactListRow) panel.ContactRow {
	return panel.ContactRow{
		ID:               r.ID,
		AccountID:        r.AccountID,
		EntityCode:       deref(r.EntityCode),
		Name:             fullName(r.FirstName, r.LastName),
		Village:          r.VillageName,
		PositionCategory: deref(r.PositionCategory),
		ContactRole:      deref(r.ContactRole),
		Phone:            maskPhone(ctx, deref(r.MobilePhone)),
		IsPrimary:        r.IsPrimaryContact,
		IsTechnical:      r.IsTechnicalContact,
		EmailOptOut:      r.EmailOptOut,
		DoNotContact:     r.DoNotContact,
	}
}

// contactDetailView merakit data detail lengkap + terapkan F4. Menerima ctx untuk
// membaca business_role aktor (dasar masking). base = prefix URL workspace,
// accountBase = URL desa induk (untuk tautan kembali & aksi). villageName = nama
// desa induk (kartu Identitas + breadcrumb). names = peta user_id→nama untuk
// Owner/Dibuat/Diubah; reportsToName = nama atasan (kontak lain di desa yang sama,
// sudah diresolusi handler). Kolom "Terakhir Dihubungi/Aktivitas" ditunda modul
// Activities → "" (view: "—").
func (h *Handler) contactDetailView(ctx context.Context, base, accountBase, villageName string, c db.Contact, names map[int64]string, reportsToName string) panel.ContactDetailView {
	return panel.ContactDetailView{
		Base:        base,
		AccountBase: accountBase,
		ID:          c.ID,
		AccountID:   c.AccountID,
		EntityCode:  deref(c.EntityCode),
		Name:        contactFullName(c),
		FirstName:   c.FirstName,
		LastName:    deref(c.LastName),
		Salutation:  deref(c.Salutation),
		JobTitle:    deref(c.JobTitle),
		VillageName: villageName,

		ReportsToID:   derefID(c.ReportsToID),
		ReportsToName: reportsToName,
		OwnerName:     memberName(names, c.ContactOwner),

		PositionCategory: deref(c.PositionCategory),
		ContactRole:      deref(c.ContactRole),
		TermPeriod:       deref(c.TermPeriod),
		IsPrimary:        c.IsPrimaryContact,
		IsTechnical:      c.IsTechnicalContact,

		// F4: HP/WhatsApp = PII pribadi perangkat desa (Sales saja utuh); telepon
		// kantor = nomor kelembagaan → tak disamarkan.
		MobilePhone: maskPhone(ctx, deref(c.MobilePhone)),
		OfficePhone: deref(c.OfficePhone),
		Email:       deref(c.Email),

		PreferredChannel: deref(c.PreferredChannel),
		MailingAddress:   deref(c.MailingAddress),
		City:             deref(c.City),
		PostalCode:       deref(c.PostalCode),

		EmailOptOut:  c.EmailOptOut,
		DoNotContact: c.DoNotContact,

		CreatedByName: memberName(names, c.CreatedBy),
		CreatedAt:     fmtDateTime(c.CreatedAt),
		UpdatedByName: memberName(names, c.UpdatedBy),
		UpdatedAt:     fmtDateTime(c.UpdatedAt),

		CanWrite:   canWriteContacts(ctx),
		Activities: h.activitiesTimelineFor(ctx, base, "contact", c.ID, canWriteSalesActivityPerm(ctx)),
	}
}
