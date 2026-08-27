package main

import (
	"context"
	"fmt"
	"math/rand"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// accounts.go — 40 "desa" (Account), hub seluruh modul CRM. Nama desa gabungan
// prefix+suffix umum di Indonesia (bukan daftar nyata — hindari klaim atas desa
// sungguhan) dipasangkan ke satu kecamatan REAL dari pickDistricts. Semua kolom
// nullable diisi supaya kolom di UI/laporan tak pernah kosong-total.

var villagePrefixes = []string{
	"Suka", "Mekar", "Cipayung", "Marga", "Karang", "Tanjung", "Sindang",
	"Cibeunying", "Wonosari", "Sumber", "Bumi", "Giri", "Pasir", "Cinta",
	"Rawa", "Kebon", "Batu", "Tegal", "Sido", "Panca",
}

var villageSuffixes = []string{
	"maju", "sari", "mulya", "asih", "jaya", "asri", "indah", "makmur",
	"agung", "rejo", "wangi", "lestari", "utama", "harja", "tirta",
}

var villageAddresses = []string{
	"Jl. Raya Desa No. 1", "Jl. Poros Kecamatan Km 3", "Jl. Merdeka No. 12",
	"Jl. Balai Desa No. 5", "Jl. Pasar Desa No. 8",
}

var territories = []string{"Wilayah Barat", "Wilayah Tengah", "Wilayah Timur"}

// accountInfo = ID + account_type — dioper ke domain lain (contacts butuh
// type utk menentukan jumlah kontak; deals/quotes/dst cukup pakai .ID).
type accountInfo struct {
	ID   int64
	Type string
}

// seedAccounts membuat 40 desa: 28 customer / 8 prospect / 4 former_customer
// (weightedPick), tersebar merata ke semua district yg tersedia.
func seedAccounts(ctx context.Context, q *db.Queries, tenantID int64, tag string, rng *rand.Rand, owner *int64, districts []district) ([]accountInfo, error) {
	const total = 40
	typeWeights := []weighted[string]{
		{"customer", 28}, {"prospect", 8}, {"former_customer", 4},
	}
	statusWeights := []weighted[string]{
		{"Desa", 30}, {"Kelurahan", 6}, {"Nagari", 3}, {"Gampong", 1},
	}
	classWeights := []weighted[string]{
		{"Mandiri", 4}, {"Maju", 10}, {"Berkembang", 18},
		{"Tertinggal", 6}, {"Sangat Tertinggal", 2},
	}

	// Kombinasi prefix×suffix di-shuffle sekali lalu diambil berurutan — 40
	// nama TANPA duplikat (300 kombinasi tersedia), bukan pick() acak per-desa
	// yang bisa mengulang nama yang sama dan terasa tidak nyata.
	names := shuffledVillageNames(rng)

	var out []accountInfo
	for i := 0; i < total; i++ {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
		if err != nil {
			return nil, fmt.Errorf("kode entitas: %w", err)
		}
		d := districts[i%len(districts)]
		name := names[i]
		// Pseudo-Kemendagri: 3 segmen depan = kode kecamatan ASLI (d.Code, mis.
		// "32.01.01"), segmen ke-4 = urutan desa fiktif (lihat desaSeqOffset).
		villageCode := fmt.Sprintf("%s.%04d", d.Code, (desaSeqOffset(tag)+i+1)%10000)
		accType := weightedPick(rng, typeWeights)

		var accOwner, csm *int64
		if rng.Intn(100) < 75 { // ~75% ber-owner — sisanya sengaja NULL (keputusan disetujui).
			accOwner = owner
		}
		if accType != "former_customer" && rng.Intn(100) < 70 {
			csm = owner
		}

		// Rentang APBDes realistis: Rp 500 juta - Rp 1,5 miliar.
		budget, err := num(fmt.Sprintf("%d.00", 500_000_000+rng.Intn(1_000_000_001)))
		if err != nil {
			return nil, err
		}

		population := int32(400 + rng.Intn(8601))
		hamlets := int32(3 + rng.Intn(10))
		postal := fmt.Sprintf("%05d", 40000+rng.Intn(30000))
		phone := fmt.Sprintf("0813%08d", rng.Intn(100000000))
		office := fmt.Sprintf("021%07d", rng.Intn(10000000))
		email := fmt.Sprintf("kantor.%s@%s.desa.go.id", villageSlug(name, i), tag)
		address := pick(rng, villageAddresses)

		website := pickOrNil(rng, 35, []string{
			fmt.Sprintf("https://%s.desa.id", villageSlug(name, i)),
		})
		territory := pickOrNil(rng, 60, territories)

		acc, err := q.CreateAccount(ctx, db.CreateAccountParams{
			TenantID:              tenantID,
			EntityCode:            &code,
			VillageName:           name,
			VillageCode:           &villageCode,
			AccountType:           accType,
			AccountOwner:          accOwner,
			AssignedCsm:           csm,
			Website:               website,
			DistrictID:            &d.ID,
			VillageAddress:        &address,
			PostalCode:            &postal,
			Territory:             territory,
			VillageStatus:         ptr(weightedPick(rng, statusWeights)),
			VillageClassification: ptr(weightedPick(rng, classWeights)),
			Population:            &population,
			HamletsCount:          &hamlets,
			VillageBudget:         budget,
			ContactPhone:          &phone,
			OfficePhone:           &office,
			OfficeEmail:           &email,
			CreatedBy:             owner,
		})
		if err != nil {
			return nil, fmt.Errorf("desa #%d: %w", i+1, err)
		}
		out = append(out, accountInfo{ID: acc.ID, Type: accType})
	}
	return out, nil
}

// shuffledVillageNames menghasilkan slice "Desa {Prefix}{Suffix}" dari SELURUH
// kombinasi prefix×suffix (300), diacak Fisher-Yates dgn rng yang sama dgn
// pemanggil (deterministik per tag run, tapi beda tiap run).
func shuffledVillageNames(rng *rand.Rand) []string {
	var combos []string
	for _, p := range villagePrefixes {
		for _, s := range villageSuffixes {
			combos = append(combos, "Desa "+p+s)
		}
	}
	rng.Shuffle(len(combos), func(i, j int) { combos[i], combos[j] = combos[j], combos[i] })
	return combos
}

// villageSlug menghasilkan slug pendek deterministik dari nama+index — dipakai
// utk website/email supaya tetap konsisten dgn nama desa tanpa tabrakan.
func villageSlug(name string, idx int) string {
	slug := ""
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			slug += string(r)
		case r >= 'A' && r <= 'Z':
			slug += string(r + 32)
		}
	}
	return fmt.Sprintf("%s%d", slug, idx)
}
