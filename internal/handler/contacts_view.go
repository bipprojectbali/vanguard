package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts_view.go — pemetaan model DB → data siap-render view (murni-data).
// Penyamaran Field-Level Security (F4) dilakukan DI SINI, di handler, sebelum
// nilai menyentuh view: yang tak berhak TAK PERNAH menerima nomor aslinya (lihat
// fls.go). Kontak punya TIGA nomor (HP, WhatsApp, kantor) — HP & WhatsApp adalah
// kanal pribadi perangkat desa (PII, disamarkan bagi non-Sales); telepon kantor
// = nomor kelembagaan, tak disamarkan (sejajar OfficePhone di accounts).

// contactRowView memetakan satu baris daftar kontak. Nomor TIDAK ikut di baris
// daftar (PII; hanya relevan di detail/form), jadi tak ada yang perlu disamarkan
// di sini. Penanda primary/opt-out ikut agar terlihat sekilas.
func contactRowView(c db.Contact) panel.ContactRow {
	return panel.ContactRow{
		ID:               c.ID,
		AccountID:        c.AccountID,
		Name:             contactFullName(c),
		JobTitle:         deref(c.JobTitle),
		PositionCategory: deref(c.PositionCategory),
		ContactRole:      deref(c.ContactRole),
		IsPrimary:        c.IsPrimaryContact,
		IsTechnical:      c.IsTechnicalContact,
		EmailOptOut:      c.EmailOptOut,
		DoNotContact:     c.DoNotContact,
	}
}

// contactDetailView merakit data detail lengkap + terapkan F4. Menerima ctx untuk
// membaca business_role aktor (dasar masking). base = prefix URL workspace,
// accountBase = URL desa induk (untuk tautan kembali & aksi).
func (h *Handler) contactDetailView(ctx context.Context, base, accountBase string, c db.Contact) panel.ContactDetailView {
	br := session.BusinessRole(ctx)
	return panel.ContactDetailView{
		Base:        base,
		AccountBase: accountBase,
		ID:          c.ID,
		AccountID:   c.AccountID,
		Name:        contactFullName(c),
		Salutation:  deref(c.Salutation),
		JobTitle:    deref(c.JobTitle),

		PositionCategory: deref(c.PositionCategory),
		ContactRole:      deref(c.ContactRole),
		TermPeriod:       deref(c.TermPeriod),
		IsPrimary:        c.IsPrimaryContact,
		IsTechnical:      c.IsTechnicalContact,

		// F4: HP & WhatsApp = PII pribadi perangkat desa (Sales saja utuh);
		// telepon kantor = nomor kelembagaan → tak disamarkan.
		MobilePhone:    maskPhone(deref(c.MobilePhone), br),
		WhatsappNumber: maskPhone(deref(c.WhatsappNumber), br),
		OfficePhone:    deref(c.OfficePhone),
		Email:          deref(c.Email),

		PreferredChannel: deref(c.PreferredChannel),
		MailingAddress:   deref(c.MailingAddress),
		City:             deref(c.City),
		PostalCode:       deref(c.PostalCode),

		EmailOptOut:  c.EmailOptOut,
		DoNotContact: c.DoNotContact,

		CanWrite: canWriteContacts(ctx),
	}
}

// contactFullName menggabungkan nama depan + belakang. last_name opsional; nama
// depan wajib (NOT NULL), jadi hasil tak pernah kosong.
func contactFullName(c db.Contact) string {
	if c.LastName != nil && *c.LastName != "" {
		return c.FirstName + " " + *c.LastName
	}
	return c.FirstName
}

// canViewContacts = gerbang READ modul Kontak (F2). Sumber tunggal untuk gate
// halaman daftar/detail; objek Casbin "crm:contacts" disebut di satu tempat.
func canViewContacts(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:contacts", "read")
}

// canWriteContactsPerm = izin F2 mentah (CanBusiness write) tanpa mempertimbangkan
// arsip. Support (read saja) ditolak menulis; deny-default untuk role "".
func canWriteContactsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:contacts", "write")
}

// canWriteContacts = gerbang aksi tombol di view. Izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper sebagai bool — view tak memanggil
// authz.
func canWriteContacts(ctx context.Context) bool {
	return canWriteContactsPerm(ctx) && !IsReadOnly(ctx)
}

// contactsMsg memetakan ?ok= → pesan sukses. Dipisah dari wsErrMsg agar alert
// sukses & galat tak pernah tertukar variannya.
func contactsMsg(code string) string {
	switch code {
	case "created":
		return "Kontak ditambahkan."
	case "saved":
		return "Perubahan kontak disimpan."
	case "primary":
		return "Kontak utama diperbarui."
	case "deleted":
		return "Kontak dihapus."
	default:
		return ""
	}
}
