package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// tickets_sort_more_test.go — lanjutan tickets_sort_test.go
// (dipecah krn ambang File Health; lihat komentar file itu utk konteks BL-157h).

// ── Status (t.status RAW — beda Renewals BL-157g yang eksklusi krn derivasi) ──

func TestTickets_SortStatusAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accSelesai := env.seedAccount(t, "Desa TStSelesai", &uid, nil, nil)
	accBaru := env.seedAccount(t, "Desa TStBaru", &uid, nil, nil)
	accDiproses := env.seedAccount(t, "Desa TStDiproses", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accSelesai.ID, Status: "selesai"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accBaru.ID, Status: "baru"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accDiproses.ID, Status: "diproses"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "status", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iB := strings.Index(body, "Desa TStBaru")
	iD := strings.Index(body, "Desa TStDiproses")
	iS := strings.Index(body, "Desa TStSelesai")
	if iB < 0 || iD < 0 || iS < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iB < iD && iD < iS) {
		t.Errorf("dir=asc harus urut baru < diproses < selesai (alfabetis), dapat posisi %d/%d/%d", iB, iD, iS)
	}
}

func TestTickets_SortStatusDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accSelesai := env.seedAccount(t, "Desa TStDSelesai", &uid, nil, nil)
	accBaru := env.seedAccount(t, "Desa TStDBaru", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accSelesai.ID, Status: "selesai"})
	env.seedTicketFull(t, ticketFullParams{AccountID: accBaru.ID, Status: "baru"})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "status", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iB := strings.Index(body, "Desa TStDBaru")
	iS := strings.Index(body, "Desa TStDSelesai")
	if iB < 0 || iS < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iS < iB) {
		t.Errorf("dir=desc harus urut selesai < baru (alfabetis terbalik), dapat posisi %d/%d", iS, iB)
	}
}

// ── Agen (u.name via LEFT JOIN users, nullable text — PLAIN, bukan COALESCE) ──

func TestTickets_SortAgentAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	agentA := env.seedMember(t, "agent-ta@x", "member", 0).ID
	agentZ := env.seedMember(t, "agent-tz@x", "member", 0).ID
	accA := env.seedAccount(t, "Desa TAAwal", &uid, nil, nil)
	accZ := env.seedAccount(t, "Desa TAZebra", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa TANull", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accA.ID, AssignedTo: &agentA})
	env.seedTicketFull(t, ticketFullParams{AccountID: accZ.ID, AssignedTo: &agentZ})
	env.seedTicketFull(t, ticketFullParams{AccountID: accN.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "agent", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa TAAwal")
	iZ := strings.Index(body, "Desa TAZebra")
	iN := strings.Index(body, "Desa TANull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut agentA < agentZ < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestTickets_SortAgentDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	agentA := env.seedMember(t, "agent-tda@x", "member", 0).ID
	agentZ := env.seedMember(t, "agent-tdz@x", "member", 0).ID
	accA := env.seedAccount(t, "Desa TADAwal", &uid, nil, nil)
	accZ := env.seedAccount(t, "Desa TADZebra", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa TADNull", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accA.ID, AssignedTo: &agentA})
	env.seedTicketFull(t, ticketFullParams{AccountID: accZ.ID, AssignedTo: &agentZ})
	env.seedTicketFull(t, ticketFullParams{AccountID: accN.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "agent", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa TADAwal")
	iZ := strings.Index(body, "Desa TADZebra")
	iN := strings.Index(body, "Desa TADNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < agentZ < agentA, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

// Pagination representatif utk cursor bertipe nullable text (pageCursorTextNullable).
func TestTickets_SortAgentPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa TAPag" + padded(i)
		agent := env.seedMember(t, "agent-tapag"+padded(i)+"@x", "member", 0).ID
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, AssignedTo: &agent})
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "agent", "asc"), env.h.TicketsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("agen terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/tickets")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/tickets?tab=&sort=agent&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.TicketsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("agen terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── SLA (sla_deadline_at RAW timestamptz, nullable) ──

func TestTickets_SortSlaAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accSoon := env.seedAccount(t, "Desa TSSoon", &uid, nil, nil)
	accLater := env.seedAccount(t, "Desa TSLater", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa TSNull", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accSoon.ID, SlaDeadlineAt: now.Add(1 * time.Hour)})
	env.seedTicketFull(t, ticketFullParams{AccountID: accLater.ID, SlaDeadlineAt: now.Add(48 * time.Hour)})
	env.seedTicketFull(t, ticketFullParams{AccountID: accNull.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "sla", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iSoon := strings.Index(body, "Desa TSSoon")
	iLater := strings.Index(body, "Desa TSLater")
	iNull := strings.Index(body, "Desa TSNull")
	if iSoon < 0 || iLater < 0 || iNull < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iSoon < iLater && iLater < iNull) {
		t.Errorf("dir=asc harus urut Soon < Later < NULL(akhir), dapat posisi %d/%d/%d", iSoon, iLater, iNull)
	}
}

func TestTickets_SortSlaDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accSoon := env.seedAccount(t, "Desa TSDSoon", &uid, nil, nil)
	accLater := env.seedAccount(t, "Desa TSDLater", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa TSDNull", &uid, nil, nil)

	env.seedTicketFull(t, ticketFullParams{AccountID: accSoon.ID, SlaDeadlineAt: now.Add(1 * time.Hour)})
	env.seedTicketFull(t, ticketFullParams{AccountID: accLater.ID, SlaDeadlineAt: now.Add(48 * time.Hour)})
	env.seedTicketFull(t, ticketFullParams{AccountID: accNull.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "sla", "desc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iSoon := strings.Index(body, "Desa TSDSoon")
	iLater := strings.Index(body, "Desa TSDLater")
	iNull := strings.Index(body, "Desa TSDNull")
	if iSoon < 0 || iLater < 0 || iNull < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iNull < iLater && iLater < iSoon) {
		t.Errorf("dir=desc harus urut NULL(awal) < Later < Soon, dapat posisi %d/%d/%d", iNull, iLater, iSoon)
	}
}

// Pagination representatif utk cursor bertipe nullable timestamptz — presisi
// PENUH (bukan dateTimeLayout menit-saja) via ticketSlaCursorVal/Str.
func TestTickets_SortSlaPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa TSPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID, SlaDeadlineAt: now.Add(time.Duration(i) * time.Hour)})
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "sla", "asc"), env.h.TicketsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("SLA terjauh (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/tickets")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/tickets?tab=&sort=sla&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.TicketsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("SLA terjauh harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── ?sort= tak dikenal → fallback default (created_at DESC) ──

func TestTickets_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa TFallback", &uid, nil, nil)
	env.seedTicketFull(t, ticketFullParams{AccountID: acc.ID})

	rec := env.runAccount(uid, "owner", "manager", ticketsSortReq("", "resolved_at", "asc"), env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Desa TFallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}
