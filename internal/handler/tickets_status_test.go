package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_status_test.go — transisi status & filter daftar Tickets, dipisah
// dari tickets_test.go (ukuran file). Konteks/sumbu sama; lihat doc di sana.

// --- status update -------------------------------------------------------

// TestTickets_StatusUpdateSelesai: support mengubah status ke 'selesai' →
// 303 ok=updated, resolved_at terisi (diset SQL CASE WHEN), audit
// ticket.status_update tercatat.
func TestTickets_StatusUpdateSelesai(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Status", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Selesai")

	form := url.Values{"status": {"selesai"}}
	req := accountsReq(http.MethodPost, "/w/test/tickets/"+itoa(tk.ID)+"/status", form, itoa(tk.ID))
	rec := env.runAccount(uid, "owner", "support", req, env.h.TicketUpdateStatus)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus ok=updated, got %q", loc)
	}

	got, err := env.q.GetTicket(t.Context(), tk.ID)
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if got.Status != "selesai" {
		t.Errorf("status harus 'selesai', got %q", got.Status)
	}
	if !got.ResolvedAt.Valid {
		t.Error("resolved_at harus terisi saat status=selesai")
	}
	env.assertAudited(t, "ticket.status_update")
}

// TestTickets_StatusUpdateRejectInvalid: status yang bukan enum valid ditolak
// → redirect err=status; baris di DB tak berubah.
func TestTickets_StatusUpdateRejectInvalid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Invalid Status")

	form := url.Values{"status": {"pending"}} // bukan enum sahih
	req := accountsReq(http.MethodPost, "/w/test/tickets/"+itoa(tk.ID)+"/status", form, itoa(tk.ID))
	rec := env.runAccount(uid, "owner", "support", req, env.h.TicketUpdateStatus)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "status") {
		t.Errorf("redirect harus mengandung 'status', got %q (code %d)", loc, rec.Code)
	}
	if got, _ := env.q.GetTicket(t.Context(), tk.ID); got.Status != "baru" {
		t.Errorf("status tak boleh berubah dari update invalid, got %q", got.Status)
	}
}

// postStatus mengirim POST /tickets/{id}/status sebagai Support (canWrite) dan
// mengembalikan recorder — helper untuk uji transisi fase (BL-38).
func (e *testEnv) postStatus(t *testing.T, uid, ticketID int64, status string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"status": {status}}
	req := accountsReq(http.MethodPost, "/w/test/tickets/"+itoa(ticketID)+"/status", form, itoa(ticketID))
	return e.runAccount(uid, "owner", "support", req, e.h.TicketUpdateStatus)
}

// TestTickets_StatusTransitionsHappyPath: 4 fase inti BL-38 mengalir penuh
// baru → diproses → menunggu → diproses → selesai. Tiap transisi 303 ok=updated;
// resolved_at terisi HANYA di fase selesai (invarian resolved_at IFF selesai).
func TestTickets_StatusTransitionsHappyPath(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Transisi", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Transisi")

	steps := []struct {
		status       string
		wantResolved bool
	}{
		{"diproses", false},
		{"menunggu", false},
		{"diproses", false},
		{"selesai", true},
	}
	for _, s := range steps {
		rec := env.postStatus(t, uid, tk.ID, s.status)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("transisi → %q: status = %d, want 303; body:\n%s", s.status, rec.Code, rec.Body.String())
		}
		got, err := env.q.GetTicket(t.Context(), tk.ID)
		if err != nil {
			t.Fatalf("GetTicket setelah %q: %v", s.status, err)
		}
		if got.Status != s.status {
			t.Errorf("status = %q, want %q", got.Status, s.status)
		}
		if got.ResolvedAt.Valid != s.wantResolved {
			t.Errorf("fase %q: resolved_at.Valid = %v, want %v", s.status, got.ResolvedAt.Valid, s.wantResolved)
		}
	}
}

// TestTickets_ReopenClearsResolvedAt: Buka Ulang (Selesai→Diproses) WAJIB
// meng-NULL-kan resolved_at — tiket dibuka ulang tak boleh simpan resolved_at
// basi (BL-38, query UpdateTicketStatus ELSE NULL).
func TestTickets_ReopenClearsResolvedAt(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reopen", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Reopen")

	// Selesaikan dulu → resolved_at terisi.
	if rec := env.postStatus(t, uid, tk.ID, "selesai"); rec.Code != http.StatusSeeOther {
		t.Fatalf("selesai: status = %d, want 303", rec.Code)
	}
	sel, _ := env.q.GetTicket(t.Context(), tk.ID)
	if !sel.ResolvedAt.Valid {
		t.Fatal("prasyarat: resolved_at harus terisi saat selesai")
	}

	// Buka ulang → diproses.
	if rec := env.postStatus(t, uid, tk.ID, "diproses"); rec.Code != http.StatusSeeOther {
		t.Fatalf("reopen: status = %d, want 303", rec.Code)
	}
	got, _ := env.q.GetTicket(t.Context(), tk.ID)
	if got.Status != "diproses" {
		t.Errorf("status setelah reopen = %q, want 'diproses'", got.Status)
	}
	if got.ResolvedAt.Valid {
		t.Error("resolved_at harus di-NULL-kan saat tiket dibuka ulang")
	}
}

// TestTicketsList_FilterTabDiprosesMenunggu: filter tab fase baru diproses &
// menunggu (BL-38) menyaring persis — masing-masing hanya mengembalikan tiket
// pada fase itu.
func TestTicketsList_FilterTabDiprosesMenunggu(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Filter", &uid, nil, nil)
	tkD := env.seedTicketRow(t, acc.ID, "Tiket Diproses")
	tkM := env.seedTicketRow(t, acc.ID, "Tiket Menunggu")
	_ = env.seedTicketRow(t, acc.ID, "Tiket Baru") // tetap 'baru'

	if rec := env.postStatus(t, uid, tkD.ID, "diproses"); rec.Code != http.StatusSeeOther {
		t.Fatalf("set diproses: %d", rec.Code)
	}
	// menunggu hanya sah dari diproses (UI), tapi DB CHECK domain saja — langsung set.
	if rec := env.postStatus(t, uid, tkM.ID, "diproses"); rec.Code != http.StatusSeeOther {
		t.Fatalf("set diproses(M): %d", rec.Code)
	}
	if rec := env.postStatus(t, uid, tkM.ID, "menunggu"); rec.Code != http.StatusSeeOther {
		t.Fatalf("set menunggu: %d", rec.Code)
	}

	for _, tc := range []struct {
		status string
		wantID int64
	}{
		{"diproses", tkD.ID},
		{"menunggu", tkM.ID},
	} {
		at, id := firstPageCursor()
		rows, err := env.q.ListTickets(t.Context(), db.ListTicketsParams{
			CursorCreatedAt: at, CursorID: id,
			ScopeAll:     true,
			FilterStatus: tc.status,
			PageSize:     100,
		})
		if err != nil {
			t.Fatalf("ListTickets(%q): %v", tc.status, err)
		}
		if len(rows) != 1 {
			t.Fatalf("filter %q: got %d baris, want 1", tc.status, len(rows))
		}
		if rows[0].ID != tc.wantID {
			t.Errorf("filter %q: id = %d, want %d", tc.status, rows[0].ID, tc.wantID)
		}
		if rows[0].Status != tc.status {
			t.Errorf("filter %q: status baris = %q", tc.status, rows[0].Status)
		}
	}
}

// seedTicketRowWithDeadline = seedTicketRow + sla_deadline_at.
// Dipakai untuk menguji KPI SLA-at-risk / breached dengan nilai deadline spesifik.
func (e *testEnv) seedTicketRowWithDeadline(t *testing.T, accountID int64, subject string, deadline pgtype.Timestamptz) db.Ticket {
	t.Helper()
	tk, err := e.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:      e.tenantID,
		AccountID:     accountID,
		Subject:       subject,
		Priority:      "sedang",
		SlaDeadlineAt: deadline,
	})
	if err != nil {
		t.Fatalf("seed ticket dengan deadline %q: %v", subject, err)
	}
	return tk
}
