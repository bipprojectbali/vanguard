package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_view.go — whitelist kolom sortable, halaman 403, dan
// pemetaan baris tabel untuk ActivitiesList (sales_activities_page.go).
// Dipisah krn ambang File Health (Route/Handler 150 baris).

// activitySortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157j: 5 kolom tabel Sales Activities). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
// Target sengaja tak masuk: komposit (tipe+id), bukan skalar tunggal.
// "date" (created_at) SUDAH jadi sumbu default (DESC) tanpa ?sort= —
// dimasukkan whitelist toh supaya user bisa FLIP ke ASC & mengarahkan panah
// aktif eksplisit di header, lewat ListActivitiesSortByDate (arah dinamis).
var activitySortableColumns = map[string]bool{
	"kind":    true,
	"subject": true,
	"owner":   true,
	"status":  true,
	"date":    true,
}

// renderActivitiesForbidden — 403 + penjelasan bagi anggota tanpa izin
// crm:sales_activity. currentPath via activityCurrentPathFromQuery (BL-161): jalur
// ini juga dicapai dari ActivityDetail (klik baris /activity-log tanpa akses
// Sales Activities, mis. CS) — tanpa penanda "?from=log", sidebar akan menyala
// item "Sales Activities" yang Disabled untuk peran itu.
func (h *Handler) renderActivitiesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Sales Activities", activityCurrentPathFromQuery(r),
		panel.SalesForbidden("Sales Activities"))
}

// activityRowView memetakan satu aktivitas → baris tabel. Target = tipe+id (tanpa
// lookup nama → hindari N+1, rule 13). Status hanya untuk Task ("" untuk call/note
// → badge "—"). Tanggal = created_at lokal (gotcha #14: simpan UTC, tampil lokal).
// Context diisi agar AllActivitiesList dapat menampilkan kolom Konteks;
// diabaikan di Sales Activities (kolom tak ada di tabel itu).
func activityRowView(a db.Activity, names map[int64]string) panel.ActivityRow {
	return panel.ActivityRow{
		ID:         a.ID,
		Kind:       a.Kind,
		Subject:    a.Subject,
		TargetType: a.TargetType,
		TargetID:   a.TargetID,
		Owner:      ownerName(a.OwnerID, names),
		Status:     deref(a.Status),
		Created:    fmtLocal(a.CreatedAt),
		Context:    deref(a.ActivityContext),
	}
}
