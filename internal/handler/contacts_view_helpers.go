package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_view_helpers.go — helper nama & format (contactFullName/fullName/
// derefID/fmtDateTime), gerbang izin baca/tulis Kontak (canViewContacts/
// canWriteContacts*), dan pemetaan pesan (contactsMsg). Dipisah dari
// contacts_view.go (pembangun view baris & detail) agar keduanya di bawah
// ambang tipe Route/Handler (150).
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
