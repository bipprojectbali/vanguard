package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_sort_test.go — BL-157h: sort per kolom daftar Tickets (6 kolom:
// village/subject/priority/status/agent/sla). Mirror
// subscriptions_renewals_sort_test.go (BL-157g), diadaptasi utk dua hal yang
// TAK ADA di Renewals: (1) F3 ownership Tickets 3-kolom (account_owner ATAU
// assigned_csm ATAU backup_csm — bukan satu kolom owner), (2) filter tab 3-flag
// (filter_status/filter_sla_breached/filter_sla_at_risk) — risiko sama
// window_filter Renewals: klon query sort baru rawan filter tercecer.

// ticketFullParams = kontrol PENUH kolom seeding (beda dari seedTicketRow yang
// kaku status='baru'/priority='sedang') — dibutuhkan agar sort asc/desc,
// nulls-first/last (agent, sla), dan status non-default dapat diuji.
type ticketFullParams struct {
	AccountID     int64
	Subject       string // default "Tiket Sort"
	Priority      string // default "sedang"
	AssignedTo    *int64
	SlaDeadlineAt time.Time // zero => NULL
	Status        string    // default "baru"
}

func (e *testEnv) seedTicketFull(t *testing.T, p ticketFullParams) db.Ticket {
	t.Helper()
	subject := p.Subject
	if subject == "" {
		subject = "Tiket Sort"
	}
	priority := p.Priority
	if priority == "" {
		priority = "sedang"
	}
	var sla pgtype.Timestamptz
	if !p.SlaDeadlineAt.IsZero() {
		sla = pgtype.Timestamptz{Time: p.SlaDeadlineAt, Valid: true}
	}
	tk, err := e.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:      e.tenantID,
		AccountID:     p.AccountID,
		Subject:       subject,
		Priority:      priority,
		AssignedTo:    p.AssignedTo,
		SlaDeadlineAt: sla,
	})
	if err != nil {
		t.Fatalf("seed ticket full: %v", err)
	}
	if p.Status != "" && p.Status != "baru" {
		tk, err = e.q.UpdateTicketStatus(t.Context(), db.UpdateTicketStatusParams{
			Status:     p.Status,
			AssignedTo: p.AssignedTo,
			ID:         tk.ID,
		})
		if err != nil {
			t.Fatalf("seed ticket full: update status: %v", err)
		}
	}
	return tk
}

func ticketsSortReq(tab, sort, dir string) *http.Request {
	target := "/w/test/tickets?tab=" + tab + "&sort=" + sort + "&dir=" + dir
	return accountsReq(http.MethodGet, target, nil, "")
}

// ── Desa (account_name, non-nullable text) ──

func TestTickets_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa TVZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa TVAwal", &uid, nil, nil)
	accM := env.seedAccount(t, "Desa TVMekar", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accZ.ID})
	env.seedTicketFull(t, ticketFullParams{AccountID: accA.ID})
	env.seedTicketFull(t, ticketFullParams{AccountID: accM.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "village", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa TVAwal")
	iM := strings.Index(body, "Desa TVMekar")
	iZ := strings.Index(body, "Desa TVZebra")
	if iA < 0 || iM < 0 || iZ < 0 {
		t.Fatalf("ketiga desa harus tampil:\n%s", body)
	}
	if !(iA < iM && iM < iZ) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iA, iM, iZ)
	}
}

func TestTickets_SortVillageDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa TVDZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa TVDAwal", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accZ.ID})
	env.seedTicketFull(t, ticketFullParams{AccountID: accA.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "village", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa TVDAwal")
	iZ := strings.Index(body, "Desa TVDZebra")
	if iA < 0 || iZ < 0 {
		t.Fatalf("kedua desa harus tampil:\n%s", body)
	}
	if !(iZ < iA) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZ, iA)
	}
}

// F3 ownership representatif — kepemilikan Tickets 3-kolom (account_owner ATAU
// assigned_csm ATAU backup_csm), beda dari Renewals yang satu kolom owner.
// csm (data_scope='own') hanya melihat tiket desa yang dia bina.
func TestTickets_SortVillageRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "ticket-sort-f3@x", "member", 0).ID
	mine := env.seedAccount(t, "Desa TMine", nil, &uid, nil)
	theirs := env.seedAccount(t, "Desa TTheirs", nil, &other, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: mine.ID})
	env.seedTicketFull(t, ticketFullParams{AccountID: theirs.ID})

	rec := env.runAccount(uid, "owner", "csm", ticketsSortReq("", "village", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "TMine") {
		t.Error("csm harus melihat tiket desa binaannya walau sort aktif")
	}
	if strings.Contains(body, "TTheirs") {
		t.Error("sort tak boleh menembus F3: csm melihat tiket desa orang lain")
	}
}

// tab filter representatif — query sort BARU (klon manual) wajib TETAP
// menghormati tab aktif, bukan cuma ownership. Risiko utama BL-157h: filter
// tab tercecer saat mengkloning ORDER BY baru.
func TestTickets_SortVillageRespectsTab(t *testing.T) {
	env, uid := setupAccounts(t)
	accBaru := env.seedAccount(t, "Desa TTBaru", &uid, nil, nil)
	accSelesai := env.seedAccount(t, "Desa TTSelesai", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accBaru.ID, Status: "baru"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accSelesai.ID, Status: "selesai"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("baru", "village", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa TTBaru") {
		t.Error("tab=baru + sort=village harus tetap memuat tiket berstatus baru")
	}
	if strings.Contains(body, "Desa TTSelesai") {
		t.Error("tab=baru + sort=village TAK boleh memuat tiket selesai — filter tab tercecer di query sort")
	}
}

// Pagination representatif utk cursor bertipe text non-nullable.
func TestTickets_SortVillagePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa TVPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID})
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "village", "asc"), env.h.TicketsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("desa terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/tickets")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/tickets?tab=&sort=village&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.TicketsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("desa terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── Subjek (t.subject, non-nullable text) ──

func TestTickets_SortSubjectAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa TSubjek", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, Subject: "Zebra bermasalah"})
	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, Subject: "Awal bermasalah"})
	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, Subject: "Mekar bermasalah"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "subject", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Awal bermasalah")
	iM := strings.Index(body, "Mekar bermasalah")
	iZ := strings.Index(body, "Zebra bermasalah")
	if iA < 0 || iM < 0 || iZ < 0 {
		t.Fatalf("ketiga subjek harus tampil:\n%s", body)
	}
	if !(iA < iM && iM < iZ) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iA, iM, iZ)
	}
}

func TestTickets_SortSubjectDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa TSubjekD", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, Subject: "Zebra rusak"})
	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, Subject: "Awal rusak"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "subject", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Awal rusak")
	iZ := strings.Index(body, "Zebra rusak")
	if iA < 0 || iZ < 0 {
		t.Fatalf("kedua subjek harus tampil:\n%s", body)
	}
	if !(iZ < iA) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZ, iA)
	}
}

// ── Prioritas (t.priority, non-nullable enum text) ──

func TestTickets_SortPriorityAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accTinggi := env.seedAccount(t, "Desa TPTinggi", &uid, nil, nil)
	accRendah := env.seedAccount(t, "Desa TPRendah", &uid, nil, nil)
	accSedang := env.seedAccount(t, "Desa TPSedang", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accTinggi.ID, Priority: "tinggi"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accRendah.ID, Priority: "rendah"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accSedang.ID, Priority: "sedang"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "priority", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iR := strings.Index(body, "Desa TPRendah")
	iS := strings.Index(body, "Desa TPSedang")
	iT := strings.Index(body, "Desa TPTinggi")
	if iR < 0 || iS < 0 || iT < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iR < iS && iS < iT) {
		t.Errorf("dir=asc harus urut rendah < sedang < tinggi (alfabetis), dapat posisi %d/%d/%d", iR, iS, iT)
	}
}

func TestTickets_SortPriorityDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accTinggi := env.seedAccount(t, "Desa TPDTinggi", &uid, nil, nil)
	accRendah := env.seedAccount(t, "Desa TPDRendah", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accTinggi.ID, Priority: "tinggi"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accRendah.ID, Priority: "rendah"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "priority", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iR := strings.Index(body, "Desa TPDRendah")
	iT := strings.Index(body, "Desa TPDTinggi")
	if iR < 0 || iT < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iT < iR) {
		t.Errorf("dir=desc harus urut tinggi < rendah (alfabetis terbalik), dapat posisi %d/%d", iT, iR)
	}
}
