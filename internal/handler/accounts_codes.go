package handler

import (
	"context"
	"fmt"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// accounts_codes.go — alokasi entity_code (kode sistem) saat create desa
// (create-time only; AccountUpdate tak menyentuhnya):
//
//   - entity_code (kode sistem, mis. "DESA-001") = OTOMATIS by-default (counter
//     atomik GenerateEntityCode), tapi operator boleh MENIMPA lewat input manual.
//
// village_code (Kode Kemendagri, mis. "32.01.01.0001") TAK lagi dialokasikan di
// sini (BL-66): ia = kode ASLI master regions level 4 (Desa), diambil apa adanya
// dari Desa yang dipilih di form (AccountCreate → GetVillageRegion). Tak ada lagi
// perakitan urut <kode kecamatan>.<urut>.
//
// Keunikan entity_code dijaga index unik parsial per tenant
// (idx_accounts_entity_code). Helper di sini memilih kode yang DIPASTIKAN bebas
// SEBELUM INSERT: INSERT gagal (pelanggaran unik) membatalkan seluruh tx
// ber-tenant → tak bisa dicoba ulang di request yang sama, jadi pemilihan harus
// lewat SELECT (AccountEntityCodeExists) yang tak membatalkan tx.

// codeAllocMaxTries membatasi loop pencarian slot bebas — pengaman terhadap loop
// tak berujung bila ada anomali data (mis. index rusak). Nilainya jauh di atas
// jumlah desa realistis satu tenant; melampauinya = tanda kesalahan, bukan beban
// normal.
const codeAllocMaxTries = 10000

// allocEntityCode memilih entity_code (kode sistem) siap-simpan. override != nil
// (operator mengisi field manual) → dipakai APA ADANYA; benturan dgn kode lain
// muncul sbg pelanggaran idx_accounts_entity_code → pesan "entity_code_dup".
// override == nil → OTOMATIS: majukan counter atomik GenerateEntityCode sampai
// mendarat di kode yang belum dipakai — ini membuat jalur otomatis MELEWATI slot
// yang sudah direbut override manual (mis. seseorang menetapkan "DESA-2" manual →
// auto berikutnya melompat ke "DESA-3"), tanpa pernah menemui deadlock karena
// hanya kode terverifikasi-bebas yang di-INSERT.
func (h *Handler) allocEntityCode(ctx context.Context, tenantID int64, override *string) (string, error) {
	if override != nil {
		return *override, nil
	}
	for i := 0; i < codeAllocMaxTries; i++ {
		code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
		if err != nil {
			return "", err
		}
		exists, err := h.q(ctx).AccountEntityCodeExists(ctx, db.AccountEntityCodeExistsParams{
			TenantID:   tenantID,
			EntityCode: &code,
		})
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("accounts: gagal menemukan entity_code otomatis bebas setelah %d percobaan", codeAllocMaxTries)
}
