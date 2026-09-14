package handler

// sales_deals_stage_sequence.go — aturan URUTAN pipeline (BL-159): form ubah
// tahap tak boleh lompat. Dipisah dari sales_deals_stage.go (aksi POST) &
// sales_deals_form_options.go (enum murni) karena ini LOGIKA transisi, dipakai
// baik oleh view (opsi dropdown, dealDetailView) maupun handler (validasi
// backend, DealStage) — SATU sumber kebenaran, bukan diduplikasi.

// activeDealStages = tahap pipeline AKTIF (bukan hasil terminal) — dealStageOptions
// minus dua tahap terminal di ekor (Closed Won, Closed Lost). Basis urutan maju.
var activeDealStages = dealStageOptions[:len(dealStageOptions)-2]

// nextDealStages mengembalikan tahap SAH berikutnya dari `current` (BL-159,
// sequential-only; keputusan user 14 Sep): SATU opsi (tahap aktif berikutnya)
// untuk tahap aktif biasa; DUA opsi (Closed Won, Closed Lost) khusus dari tahap
// aktif TERAKHIR ("Negotiation") — deal hanya boleh gugur (Closed Lost) dari
// tahap terakhir sebelum ditutup, BUKAN dari tahap manapun (opsi awal "izinkan
// dari mana saja" eksplisit dibalik user). `current` sudah terminal atau tak
// dikenal → nil (form ubah tahap disembunyikan di view utk deal terminal,
// lihat canAct di sales_deals_detail.go).
func nextDealStages(current string) []string {
	for i, s := range activeDealStages {
		if s != current {
			continue
		}
		if i == len(activeDealStages)-1 {
			return []string{"Closed Won", "Closed Lost"}
		}
		return []string{activeDealStages[i+1]}
	}
	return nil
}

// isValidDealStageTransition menegakkan aturan sequential-only DI BACKEND —
// jaring terakhir, tak boleh dilewati via POST langsung walau dropdown UI
// sudah membatasi opsi. Reopen (dari stage TERMINAL kembali ke tahap aktif,
// mis. Closed Lost → Proposal) SENGAJA tak dibatasi di sini — di luar cakupan
// laporan BL-159 (soal lompat MAJU), dan sudah perilaku existing yang dijaga
// test regresi (TestDealReopen_ClearsLossReasonCode); UI pun tak pernah
// mengekspos jalur ini (modal ubah tahap disembunyikan utk deal terminal).
func isValidDealStageTransition(current, next string) bool {
	if current == "Closed Won" || current == "Closed Lost" {
		return true
	}
	for _, s := range nextDealStages(current) {
		if s == next {
			return true
		}
	}
	return false
}
