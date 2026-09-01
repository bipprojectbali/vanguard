package handler

import (
	"context"
	"strconv"
	"strings"

	"go_starter/internal/authz"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_view_helpers.go — gerbang izin baca/tulis Account (canViewAccounts /
// canWriteAccounts*), pelabelan tipe & pesan (accountTypeLabel/accountsMsg), dan
// helper format nilai opsional (int32Str/numericStr). Dipisah dari accounts_view.go
// (pembangun view baris & detail) agar keduanya di bawah ambang tipe Route/Handler (150).
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

// moneyRupiahStr = prefill untuk field UANG BULAT (moneyField + numgroup.js):
// numericStr menjaga skala kolom NUMERIC(15,2) → "7500000.00", tapi numgroup.js
// membuang SEMUA non-digit (titik desimal ikut) sehingga "7500000.00" akan
// ditampilkan & disimpan ulang jadi "750000000" (100x). Buang bagian pecahan di
// sini agar prefill = rupiah bulat murni ("7500000"), konsisten dgn formatRupiah
// (tampilan juga membuang desimal). NULL → "".
func moneyRupiahStr(n pgtype.Numeric) string {
	s := numericStr(n)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	return s
}
