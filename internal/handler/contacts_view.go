package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_view.go — pemetaan model DB → data siap-render view (murni-data).
// Penyamaran Field-Level Security (F4) dilakukan DI SINI, di handler, sebelum
// nilai menyentuh view: yang tak berhak TAK PERNAH menerima nomor aslinya (lihat
// fls.go). Kontak punya TIGA nomor (HP, WhatsApp, kantor) — HP & WhatsApp adalah
// kanal pribadi perangkat desa (PII, disamarkan bagi non-Sales); telepon kantor
// = nomor kelembagaan, tak disamarkan (sejajar OfficePhone di accounts).

// contactRowView memetakan satu baris daftar kontak SATU desa. WhatsApp ikut di
// baris (wireframe M3) tapi DISAMARKAN di sini (F4) bila aktor bukan Sales — nomor
// asli tak pernah dioper ke view. Village kosong (daftar per-desa tak berkolom
// Desa). LastActivity ditunda modul Activities → "" (view: "—").
func contactRowView(ctx context.Context, c db.Contact) panel.ContactRow {
	return panel.ContactRow{
		ID:               c.ID,
		AccountID:        c.AccountID,
		Name:             contactFullName(c),
		PositionCategory: deref(c.PositionCategory),
		ContactRole:      deref(c.ContactRole),
		Whatsapp:         maskPhone(deref(c.WhatsappNumber), session.BusinessRole(ctx)),
		IsPrimary:        c.IsPrimaryContact,
		IsTechnical:      c.IsTechnicalContact,
		EmailOptOut:      c.EmailOptOut,
		DoNotContact:     c.DoNotContact,
	}
}

// contactRowViewGlobal memetakan satu baris daftar kontak LINTAS-desa. Sama dengan
// contactRowView + kolom Desa (village_name dari JOIN accounts). Tipe baris berbeda
// (db.ListContactsRow superset db.Contact) jadi pemetaannya terpisah, bukan dipaksa
// lewat konversi.
func contactRowViewGlobal(ctx context.Context, r db.ListContactsRow) panel.ContactRow {
	return panel.ContactRow{
		ID:               r.ID,
		AccountID:        r.AccountID,
		Name:             fullName(r.FirstName, r.LastName),
		Village:          r.VillageName,
		PositionCategory: deref(r.PositionCategory),
		ContactRole:      deref(r.ContactRole),
		Whatsapp:         maskPhone(deref(r.WhatsappNumber), session.BusinessRole(ctx)),
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
	br := session.BusinessRole(ctx)
	return panel.ContactDetailView{
		Base:        base,
		AccountBase: accountBase,
		ID:          c.ID,
		AccountID:   c.AccountID,
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

		CreatedByName: memberName(names, c.CreatedBy),
		CreatedAt:     fmtDateTime(c.CreatedAt),
		UpdatedByName: memberName(names, c.UpdatedBy),
		UpdatedAt:     fmtDateTime(c.UpdatedAt),

		CanWrite: canWriteContacts(ctx),
	}
}

// contactFullName menggabungkan nama depan + belakang. last_name opsional; nama
// depan wajib (NOT NULL), jadi hasil tak pernah kosong.
func contactFullName(c db.Contact) string {
	return fullName(c.FirstName, c.LastName)
}

// fullName menggabungkan nama depan + belakang opsional — dipakai baik oleh
// db.Contact maupun baris JOIN (db.ListContactsRow) yang tak punya metode bersama.
func fullName(first string, last *string) string {
	if last != nil && *last != "" {
		return first + " " + *last
	}
	return first
}

// derefID mengembalikan 0 untuk id opsional nil (view: "tak ada atasan"), atau
// nilainya. 0 aman sebagai penanda "kosong" karena id baris selalu > 0.
func derefID(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// fmtDateTime memformat timestamptz audit ke waktu lokal aplikasi DENGAN tahun
// (beda dari fmtLocal yang untuk agregasi jam-hari saja). created_at/updated_at
// NOT NULL, jadi praktis selalu valid; "—" hanya jaga-jaga.
func fmtDateTime(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return "—"
	}
	return ts.Time.In(appTZ).Format("02 Jan 2006 15:04")
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
