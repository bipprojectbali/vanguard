package handler

import (
	"context"
	"errors"
	"fmt"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// accounts_codes.go — alokasi DUA kode desa saat create (keduanya create-time
// only; AccountUpdate tak menyentuhnya):
//
//   - village_code (Kode Kemendagri, mis. "32.01.01.0001") = OTOMATIS penuh,
//     diturunkan dari Kecamatan terpilih: <kode kecamatan>.<urut 4 digit>. Format
//     sama dgn seeder demo (cmd/seeddemo/accounts.go). Tak ada input manual.
//   - entity_code (kode sistem, mis. "DESA-001") = OTOMATIS by-default (counter
//     atomik GenerateEntityCode), tapi operator boleh MENIMPA lewat input manual.
//
// Keunikan final KEDUANYA dijaga index unik parsial per tenant (idx_accounts_code
// utk village_code, idx_accounts_entity_code utk entity_code). Helper di sini
// memilih kode yang DIPASTIKAN bebas SEBELUM INSERT: INSERT gagal (pelanggaran
// unik) membatalkan seluruh tx ber-tenant → tak bisa dicoba ulang di request yang
// sama, jadi pemilihan harus lewat SELECT (VillageCodeExists/AccountEntityCodeExists)
// yang tak membatalkan tx.

// codeAllocMaxTries membatasi loop pencarian slot bebas — pengaman terhadap loop
// tak berujung bila ada anomali data (mis. index rusak). Nilainya jauh di atas
// jumlah desa realistis satu Kecamatan; melampauinya = tanda kesalahan, bukan
// beban normal.
const codeAllocMaxTries = 10000

// errInvalidDistrict = district_id yang dikirim bukan Kecamatan (level 3) sah —
// dipetakan pemanggil ke galat "district_id" (bukan "failed"): sebabnya di sisi
// input user yang bisa diperbaiki, bukan kegagalan internal.
var errInvalidDistrict = errors.New("accounts: district_id bukan kecamatan sah")

// generateVillageCode merakit village_code Kemendagri otomatis <kode
// kecamatan>.<urut 4 digit> untuk tenant + Kecamatan ini. Urut mulai dari
// MAX+1 atas SEMUA baris (termasuk soft-deleted → nomor tak pernah dipakai
// ulang), lalu maju melewati slot yang keburu direbut request lain
// (VillageCodeExists) sebelum dikembalikan. Kecamatan tak dikenal →
// errInvalidDistrict.
func (h *Handler) generateVillageCode(ctx context.Context, tenantID, districtID int64) (string, error) {
	dcode, err := h.q(ctx).GetDistrictCode(ctx, districtID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errInvalidDistrict
		}
		return "", err
	}

	// prefix LIKE: dcode + ".%" — titik bukan wildcard LIKE (hanya _ dan % yang
	// spesial), dan kode Kecamatan berlebar tetap (PP.KK.CC) jadi prefix tak
	// pernah cocok lintas-Kecamatan.
	prefix := dcode + ".%"
	maxSeq, err := h.q(ctx).MaxVillageSeqForPrefix(ctx, db.MaxVillageSeqForPrefixParams{
		TenantID: tenantID,
		Prefix:   &prefix,
	})
	if err != nil {
		return "", err
	}

	seq := int(maxSeq) + 1
	for i := 0; i < codeAllocMaxTries; i++ {
		candidate := fmt.Sprintf("%s.%04d", dcode, seq)
		exists, err := h.q(ctx).VillageCodeExists(ctx, db.VillageCodeExistsParams{
			TenantID:    tenantID,
			VillageCode: &candidate,
		})
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		seq++
	}
	return "", fmt.Errorf("accounts: gagal menemukan village_code bebas untuk %q setelah %d percobaan", dcode, codeAllocMaxTries)
}

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
