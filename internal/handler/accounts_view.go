package handler

import (
	"context"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_view.go — pemetaan model DB → data siap-render view (murni-data).
// Penyamaran Field-Level Security (F4) dilakukan DI SINI, di handler, sebelum
// nilai menyentuh view: yang tak berhak TAK PERNAH menerima nilai aslinya (lihat
// fls.go). View accounts hanya menerima string yang sudah diputuskan.

// accountRowView memetakan satu baris daftar. Nomor HP TIDAK ikut di baris
// daftar (PII; hanya relevan di detail), jadi tak ada yang perlu disamarkan di
// sini — masking F4 berlaku di detail. names = peta user_id→nama (dirakit sekali
// di handler) untuk kolom Owner/CSM; id tak-tertugas / tak-dikenal → "" ("—").
func accountRowView(a db.Account, names map[int64]string) panel.AccountRow {
	return panel.AccountRow{
		ID:          a.ID,
		EntityCode:  deref(a.EntityCode),
		VillageName: a.VillageName,
		VillageCode: deref(a.VillageCode),
		AccountType: accountTypeLabel(a.AccountType),
		Regency:     deref(a.Regency),
		Province:    deref(a.Province),
		OwnerName:   memberName(names, a.AccountOwner),
		CSMName:     memberName(names, a.AssignedCsm),
	}
}

// accountDetailView merakit data detail lengkap + terapkan F4. Menerima ctx untuk
// membaca business_role aktor (dasar masking). base = prefix URL workspace.
func (h *Handler) accountDetailView(ctx context.Context, base string, a db.Account) panel.AccountDetailView {
	br := session.BusinessRole(ctx)
	return panel.AccountDetailView{
		Base:        base,
		ID:          a.ID,
		EntityCode:  deref(a.EntityCode),
		VillageName: a.VillageName,
		VillageCode: deref(a.VillageCode),
		AccountType: accountTypeLabel(a.AccountType),
		Website:     deref(a.Website),
		Description: deref(a.Description),

		Province:       deref(a.Province),
		Regency:        deref(a.Regency),
		District:       deref(a.District),
		VillageAddress: deref(a.VillageAddress),
		PostalCode:     deref(a.PostalCode),
		Territory:      deref(a.Territory),

		VillageStatus:         deref(a.VillageStatus),
		VillageClassification: deref(a.VillageClassification),
		Population:            int32Str(a.Population),
		HamletsCount:          int32Str(a.HamletsCount),
		VillageBudget:         numericStr(a.VillageBudget),

		// F4: nomor HP kontak utuh HANYA Sales; lainnya tersamar. Office phone/
		// email = data kelembagaan (bukan PII pribadi kepala desa) → tak disamar.
		ContactPhone: maskPhone(deref(a.ContactPhone), br),
		OfficePhone:  deref(a.OfficePhone),
		OfficeEmail:  deref(a.OfficeEmail),

		CanWrite:   canWriteAccounts(ctx),
		Activities: h.activitiesTimelineFor(ctx, base, "account", a.ID, canWriteSalesActivityPerm(ctx)),
	}
}

// canViewAccounts = gerbang READ modul Desa (F2). Sumber tunggal untuk menu
// sidebar & gate halaman AccountsList — keduanya WAJIB menyebut objek/aksi yang
// sama, agar menu tak pernah menawarkan pintu yang lalu ditolak 403.
func canViewAccounts(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:accounts", "read")
}

// canWriteAccountsPerm = izin F2 mentah (CanBusiness write) tanpa mempertimbangkan
// arsip. Satu tempat objek Casbin "crm:accounts" disebut untuk aksi tulis, agar
// gate POST & tombol view tak pernah menyebut objek yang berbeda.
func canWriteAccountsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:accounts", "write")
}

// canWriteAccounts = gerbang aksi tombol di view detail. Izin F2 write DAN
// workspace tidak read-only (arsip). Dihitung di handler, dioper sebagai bool —
// view tak memanggil authz.
func canWriteAccounts(ctx context.Context) bool {
	return canWriteAccountsPerm(ctx) && !IsReadOnly(ctx)
}

// accountTypeLabel = nilai enum → label Bahasa Indonesia. Nilai tak dikenal
// (mustahil setelah validasi + CHECK) → mentahnya, bukan kosong.
func accountTypeLabel(t string) string {
	switch t {
	case "prospect":
		return "Prospek"
	case "customer":
		return "Pelanggan"
	case "former_customer":
		return "Mantan Pelanggan"
	default:
		return t
	}
}

// accountsMsg memetakan ?ok= → pesan sukses. Dipisah dari wsErrMsg agar alert
// sukses & galat tak pernah tertukar variannya.
func accountsMsg(code string) string {
	switch code {
	case "created":
		return "Desa ditambahkan."
	case "saved":
		return "Perubahan desa disimpan."
	case "assigned":
		return "Penugasan diperbarui."
	case "deleted":
		return "Desa dihapus."
	default:
		return ""
	}
}

// int32Str memformat *int32 opsional: nil → "". Dipakai untuk penduduk/dusun.
func int32Str(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(int64(*p), 10)
}

// numericStr memformat pgtype.Numeric untuk tampilan. Invalid (NULL) → "".
// Nilai valid dirender lewat Value() (string desimal kanonik Postgres); format
// mata uang yang lebih kaya bisa menyusul, tapi angka mentah yang benar lebih
// baik daripada pembulatan yang salah.
func numericStr(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
