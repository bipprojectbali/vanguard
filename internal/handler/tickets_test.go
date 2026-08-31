package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_test.go — Tickets / Cases (Modul 6 slice B2, wireframe 6.9).
// Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET /tickets butuh crm:tickets read — semua peran CRM
//     (admin/manager/support/sales/csm) lolos; "" ditolak 403. POST butuh
//     crm:tickets write — admin/manager/support lolos; sales & csm (read-saja)
//     & "" ditolak 403 tanpa menyentuh DB.
//   - F3 (ownership, tickets): CSM (data_scope='own') melihat HANYA tiket desa
//     binaannya; Support (data_scope='none' + canWrite) melihat SEMUA tiket via
//     override TicketsListFilterFor. Admin (data_scope='all') melihat semua.
//   - SLA snapshot: sla_deadline_at = now()+menit di-hitung HANDLER saat create,
//     bukan JOIN live ke sla_policies — nilai tersimpan saat tiket lahir.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup & helper request memakai ulang setupAccounts/accountsReq/
// runAccount. Meniru kb_articles_test.go.

// --- helper ----------------------------------------------------------------

// ticketFormValues merakit form minimal yang valid untuk create.
// priority default 'sedang'.
func ticketFormValues(accountID int64, subject string) url.Values {
	return url.Values{
		"account_id": {itoa(accountID)},
		"subject":    {subject},
		"priority":   {"sedang"},
	}
}

// seedTicketRow menaruh satu tiket langsung lewat pool (bypass handler),
// status='baru', priority='sedang' — untuk menguji UpdateTicketStatus tanpa
// merangkai TicketCreate. Mengembalikan db.Ticket utuh.
func (e *testEnv) seedTicketRow(t *testing.T, accountID int64, subject string) db.Ticket {
	t.Helper()
	tk, err := e.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:  e.tenantID,
		AccountID: accountID,
		Subject:   subject,
		Priority:  "sedang",
	})
	if err != nil {
		t.Fatalf("seed ticket %q: %v", subject, err)
	}
	return tk
}

// allTickets mendaftar seluruh tiket workspace (scope_all, tanpa filter status)
// langsung dari pool — untuk membuktikan sebuah aksi menyimpan / tak
// menyimpan baris.
func (e *testEnv) allTickets(t *testing.T) []db.ListTicketsRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListTickets(t.Context(), db.ListTicketsParams{
		CursorCreatedAt: at,
		CursorID:        id,
		ScopeAll:        true,
		FilterStatus:    "",
		PageSize:        100,
	})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestTickets_GateRead: siapa boleh MEMBUKA daftar tiket (act read).
// Semua peran CRM lolos (admin/manager/support/sales/csm); "" ditolak 403
// dengan penjelasan butuh peran CRM.
func TestTickets_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	// Seed akun agar JOIN accounts di ListTickets tidak error walau list kosong.
	acc := env.seedAccount(t, "Desa Gate", &uid, nil, nil)
	_ = env.seedTicketRow(t, acc.ID, "Tiket Gate")

	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"support", true},
		{"sales", true},
		{"csm", true},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/tickets", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.TicketsList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// --- F2: gerbang write -------------------------------------------------

// TestTickets_GateWrite: POST create butuh act write — admin/manager/support
// lolos; sales & csm (read-saja) & "" ditolak 403 tanpa menyentuh DB.
func TestTickets_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"support", true},
		{"sales", false},
		{"csm", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			acc := env.seedAccount(t, "Desa Write Gate", &uid, nil, nil)
			form := ticketFormValues(acc.ID, "Tiket Write Gate")
			req := accountsReq(http.MethodPost, "/w/test/tickets", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.TicketCreate)

			rows := env.allTickets(t)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 baris, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan apa pun, ada %d baris", c.role, len(rows))
				}
			}
		})
	}
}

// --- create ------------------------------------------------------------

// TestTickets_CreateSuccess: support create → 303 ok=created; baris tersimpan
// status='baru', subject & priority sesuai input; audit ticket.create tercatat.
func TestTickets_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Maju", &uid, nil, nil)
	form := ticketFormValues(acc.ID, "Server tidak bisa diakses")
	form.Set("priority", "tinggi")
	form.Set("description", "Server down sejak pukul 07.00.")
	req := accountsReq(http.MethodPost, "/w/test/tickets", form, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.TicketCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}

	rows := env.allTickets(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	tk := rows[0]
	if tk.Subject != "Server tidak bisa diakses" {
		t.Errorf("subject salah: %q", tk.Subject)
	}
	if tk.Priority != "tinggi" {
		t.Errorf("priority salah: %q", tk.Priority)
	}
	if tk.Status != "baru" {
		t.Errorf("tiket baru harus lahir status='baru', got %q", tk.Status)
	}
	if tk.AccountName != "Desa Maju" {
		t.Errorf("account_name salah: %q", tk.AccountName)
	}
	env.assertAudited(t, "ticket.create")
}

// TestTickets_CreateWithSLADeadline: support create dengan sla_policy_id yang
// punya resolution_target_minutes → sla_deadline_at tersimpan (Valid=true,
// snapshot saat create). Cukup uji Valid karena nilai tepat tergantung waktu
// test.
func TestTickets_CreateWithSLADeadline(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa SLA", &uid, nil, nil)

	// Seed SLA policy dengan 240 menit resolusi.
	resMinutes := int32(240)
	sla, err := env.q.CreateSLAPolicy(t.Context(), db.CreateSLAPolicyParams{
		TenantID:                env.tenantID,
		SlaName:                 "SLA Standard",
		ResolutionTargetMinutes: &resMinutes,
		IsActive:                true,
	})
	if err != nil {
		t.Fatalf("seed sla policy: %v", err)
	}

	form := ticketFormValues(acc.ID, "Tiket SLA")
	form.Set("sla_policy_id", itoa(sla.ID))
	req := accountsReq(http.MethodPost, "/w/test/tickets", form, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.TicketCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	rows := env.allTickets(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	if !rows[0].SlaDeadlineAt.Valid {
		t.Error("sla_deadline_at harus terisi (snapshot) bila sla_policy dipilih")
	}
	// Verifikasi sla_policy_id tersimpan via GetTicket (ada kolom SlaPolicyID).
	got, err := env.q.GetTicket(t.Context(), rows[0].ID)
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if got.SlaPolicyID == nil || *got.SlaPolicyID != sla.ID {
		t.Errorf("sla_policy_id harus merujuk ke policy yang dipilih, got %v", got.SlaPolicyID)
	}
}

// TestTickets_CreateRejectsInvalid: input yang melanggar validasi backend
// ditolak → redirect err + tak menyimpan apa pun ke DB.
func TestTickets_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"tanpa account_id", url.Values{"subject": {"Tiket X"}, "priority": {"sedang"}}, "err=required"},
		{"tanpa subject", url.Values{"account_id": {"1"}, "priority": {"sedang"}}, "err=required"},
		{"priority asing", url.Values{"account_id": {"1"}, "subject": {"Tiket X"}, "priority": {"urgent"}}, "err=priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/tickets", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.TicketCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allTickets(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

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
