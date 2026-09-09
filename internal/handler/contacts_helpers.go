package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// contacts_helpers.go — perkakas non-handler modul kontak: perakit path redirect,
// pemetaan Contact → prefill form (dengan masking F4), dan opsi enum dropdown +
// pemeriksa sinkron compile-time. Dipisah dari aksi agar file aksi tetap ramping.

// contactPath merakit URL absolut lengkap kontak (dipakai wsRedirectOK yang
// menerima path relatif-ke-workspace; jadi ini path relatif tanpa prefix /w/slug).
func contactPath(accountID, contactID int64) string {
	return "/accounts/" + strconv.FormatInt(accountID, 10) + "/contacts/" + strconv.FormatInt(contactID, 10)
}

// contactPathRel = alias makna-jelas untuk contactPath saat dipakai sebagai
// tujuan redirect galat (relatif-ke-workspace).
func contactPathRel(accountID, contactID int64) string {
	return contactPath(accountID, contactID)
}

// contactEditRel = path relatif form edit kontak (tujuan PRG saat validasi gagal).
func contactEditRel(accountID, contactID int64) string {
	return contactPath(accountID, contactID) + "/edit"
}

// contactFormFields memetakan Contact → nilai prefill form. F4: editor non-Sales
// menerima MASK untuk HP/WhatsApp, bukan nomor asli — nilai asli tak pernah mencapai
// browsernya (view-source pun bersih). Telepon kantor tak disamarkan.
func contactFormFields(ctx context.Context, c db.Contact) panel.ContactFormFields {
	mobile := deref(c.MobilePhone)
	if !canEditPhone(ctx) && mobile != "" {
		mobile = flsHidden // tersamar; field dikunci di view, tak ikut ter-submit
	}
	return panel.ContactFormFields{
		FirstName:          c.FirstName,
		LastName:           deref(c.LastName),
		Salutation:         deref(c.Salutation),
		JobTitle:           deref(c.JobTitle),
		PositionCategory:   deref(c.PositionCategory),
		ContactRole:        deref(c.ContactRole),
		IsPrimaryContact:   c.IsPrimaryContact,
		IsTechnicalContact: c.IsTechnicalContact,
		TermPeriod:         deref(c.TermPeriod),
		MobilePhone:        mobile,
		OfficePhone:        deref(c.OfficePhone),
		Email:              deref(c.Email),
		PreferredChannel:   deref(c.PreferredChannel),
		MailingAddress:     deref(c.MailingAddress),
		City:               deref(c.City),
		PostalCode:         deref(c.PostalCode),
		EmailOptOut:        c.EmailOptOut,
		DoNotContact:       c.DoNotContact,
	}
}

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00005; urutannya tampilan.
var (
	contactPositionOptions = []string{
		"Kepala Desa", "Sekdes", "Kaur", "Kasi",
		"Operator", "Bendahara", "BPD", "Lainnya",
	}
	contactRoleOptions = []string{
		"Decision Maker", "Influencer", "User", "Finance", "Gatekeeper",
	}
	contactChannelOptions = []string{"WhatsApp", "Telepon", "Email", "Kunjungan"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(contactPositionOptions) != len(validContactPositions) ||
		len(contactRoleOptions) != len(validContactRoles) ||
		len(contactChannelOptions) != len(validContactChannels) {
		panic("contacts: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
