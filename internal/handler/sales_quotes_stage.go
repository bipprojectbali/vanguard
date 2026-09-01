package handler

import "net/http"

// sales_quotes_stage.go — GERBANG STAGE Quote (BL-13, Opsi 2 "Seimbang"): jendela
// quoting = Qualification→Negotiation. Backend = penjaga sesungguhnya; view hanya
// menyembunyikan kontrol. Dipisah dari sales_quotes.go (file health) — dipakai
// lintas aksi create/update/status/delete + item add/update/delete.
//
// Aturan tunggal: quotableStage(stage) := stage ∈ {Qualification,Demo,Proposal,
// Negotiation}. Mutasi (buat/ubah/hapus) diizinkan HANYA saat quotable; selain itu
// READ-ONLY. Prospecting DIBLOKIR (deal baru lahir, belum terkualifikasi → penawaran
// resmi prematur; batas bawah Qualification demi realita APBDes). Terminal (Closed
// Won/Lost) READ-ONLY, bukan diblokir total: quote & item lama tetap BISA dilihat
// (arsip; Closed Won = dasar beku Subscription), tapi tak bisa buat/ubah/hapus. Satu
// aturan menutup juga kasus langka deal dipindah balik ke Prospecting saat sudah
// punya quote.

// quotableStages = himpunan tahap deal tempat Quote boleh DIBUAT/DIUBAH/DIHAPUS.
var quotableStages = map[string]struct{}{
	"Qualification": {}, "Demo": {}, "Proposal": {}, "Negotiation": {},
}

// quotableStage melaporkan apakah stage deal mengizinkan mutasi quote.
func quotableStage(stage string) bool {
	_, ok := quotableStages[stage]
	return ok
}

// stageLockMsg = alasan quote terkunci read-only (kosong bila quotable). Precompute
// di handler (bukan logika di view) → dioper ke banner. Membedakan terminal (arsip)
// vs Prospecting (belum terkualifikasi) agar pesan jujur.
func stageLockMsg(stage string) string {
	if quotableStage(stage) {
		return ""
	}
	if stage == "Closed Won" || stage == "Closed Lost" {
		return "Deal sudah ditutup (Closed Won/Lost) — quote bersifat arsip dan tak dapat diubah."
	}
	return "Deal masih di tahap Prospecting — quote baru dapat dibuat atau diubah mulai tahap Qualification."
}

// requireQuotableStage menolak mutasi quote di luar jendela quoting (BL-13). Ditolak
// → PRG ?err=quote_stage ke redirectSub (relatif /w/{slug}). Backend tetap penjaga
// walau view menyembunyikan kontrol.
func (h *Handler) requireQuotableStage(w http.ResponseWriter, r *http.Request, stage, redirectSub string) bool {
	if quotableStage(stage) {
		return true
	}
	wsRedirect(w, r, redirectSub, "quote_stage")
	return false
}
