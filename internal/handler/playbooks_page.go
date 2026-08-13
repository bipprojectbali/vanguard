package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// playbooks_page.go — HALAMAN baca katalog Playbooks (Modul 6 slice A2). Aksi
// ada di playbooks.go / playbooks_status.go. Dipisah karena halaman tumbuh
// dengan aturan LIHAT (F2 read), aksi dengan aturan TULIS (F2 write). Meniru
// sla_policies_page.go (slice A1).
//
// Katalog master bounded per-workspace (ListPlaybooksAll, TERMASUK draf) →
// TANPA keyset & TANPA F3: RLS satu-satunya pengurung.
//
// KPI eksekusi wireframe ("Sedang Berjalan"/"Selesai (Bln)"/"Tingkat Sukses")
// & kolom "Berjalan"/"Sukses" per baris DIHILANGKAN dari slice ini SENGAJA —
// ketiganya butuh tabel log eksekusi playbook yang belum ada (di luar scope
// A2, sama alasan dgn kepatuhan SLA di A1). Filter tab
// (Semua/At-Risk/Onboarding/Adopsi/Renewal) juga DIHILANGKAN — daftar
// katalog master ini bounded & kecil, filter tab menambah kompleksitas tanpa
// data eksekusi untuk membuatnya berguna (bisa disusulkan begitu tab benar-
// benar menyaring, bukan sekadar dekorasi).

// PlaybooksList — GET /w/{workspace}/playbooks. Seluruh katalog (aktif dulu
// lalu draf). Bukan pemegang peran CRM (read) → 403 + penjelasan.
func (h *Handler) PlaybooksList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewPlaybooks(ctx) {
		h.renderPlaybooksForbidden(w, r)
		return
	}
	rows, err := h.q(ctx).ListPlaybooksAll(ctx)
	if err != nil {
		h.Log.Error("playbooks: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.PlaybookRow, 0, len(rows))
	for _, p := range rows {
		items = append(items, playbookRowView(p))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Playbooks", "/playbooks", panel.PlaybookList(panel.PlaybookListView{
		Base:     base,
		CanWrite: canWritePlaybooks(ctx),
		Err:      playbooksErrMsg(r.URL.Query().Get("err")),
		Msg:      playbooksMsg(r.URL.Query().Get("ok")),
		Items:    items,
	}))
}

// renderPlaybooksForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderPlaybooksForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Playbooks", "/playbooks", panel.SalesForbidden("Playbooks"))
}

// playbookRowView memetakan satu playbook → baris tabel katalog. StepCount
// diturunkan dari steps (baris teks non-kosong) — legitimate karena murni
// dari data tersimpan, BEDA dari Berjalan/Sukses (butuh data eksekusi yang
// belum ada).
func playbookRowView(p db.Playbook) panel.PlaybookRow {
	return panel.PlaybookRow{
		ID:               p.ID,
		PlaybookName:     p.PlaybookName,
		TriggerScenario:  deref(p.TriggerScenario),
		RecommendedOwner: deref(p.RecommendedOwner),
		StepCount:        countSteps(p.Steps),
		Active:           p.IsActive,
	}
}

// countSteps menghitung baris non-kosong pada teks langkah bebas (steps).
// nil/kosong → 0. Dipakai kolom "Langkah" di katalog (angka, bukan teks
// panjang) — cermin wireframe 6.7 tanpa menyimpan langkah sebagai tabel
// terpisah (steps tetap TEXT tunggal per skema §6g).
func countSteps(steps *string) int {
	if steps == nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(*steps, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
