package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// jena_ai_tools_accounts.go — implementasi tool search_accounts &
// get_account_summary (+ jenaLoadAccount, dipakai bersama
// get_subscription_status di jena_ai_tools_pipeline.go). Dipisah dari
// jena_ai_tools.go krn ambang File Health (Route/Handler 150 baris); murni
// pindah, tak ada perubahan logika.

type jenaAccountIDInput struct {
	AccountID int64 `json:"account_id"`
}

func (h *Handler) jenaSearchAccounts(ctx context.Context, input json.RawMessage) (string, error) {
	if !canViewAccounts(ctx) {
		return `{"error":"tidak berhak melihat data desa"}`, nil
	}
	var in struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("input tidak valid: %w", err)
	}

	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	cursorAt, cursorID := firstPageCursor()
	rows, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsSales:         filter.IsOwn,
		IsCsm:           filter.IsOwn,
		Uid:             &uid,
		Search:          in.Query,
		PageSize:        jenaSearchAccountsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("cari desa: %w", err)
	}

	type row struct {
		AccountID   int64  `json:"account_id"`
		VillageName string `json:"village_name"`
	}
	results := make([]row, 0, len(rows))
	for _, a := range rows {
		results = append(results, row{AccountID: a.ID, VillageName: a.VillageName})
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(out), nil
}

// jenaLoadAccount memuat satu Account + menegakkan F2+F3 — dipakai bersama
// get_account_summary & get_subscription_status supaya keduanya tak
// menduplikasi pola gate (accounts_detail_page.go:26-58).
func (h *Handler) jenaLoadAccount(ctx context.Context, accountID int64) (db.Account, bool, error) {
	if !canViewAccounts(ctx) {
		return db.Account{}, false, nil
	}
	a, err := h.q(ctx).GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Account{}, false, nil
		}
		return db.Account{}, false, err
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.AccountOwner, a.AssignedCsm, a.BackupCsm) {
		return db.Account{}, false, nil
	}
	return a, true, nil
}

func (h *Handler) jenaGetAccountSummary(ctx context.Context, input json.RawMessage) (string, error) {
	var in jenaAccountIDInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("input tidak valid: %w", err)
	}
	a, ok, err := h.jenaLoadAccount(ctx, in.AccountID)
	if err != nil {
		return "", fmt.Errorf("ambil desa: %w", err)
	}
	if !ok {
		return `{"error":"desa tidak ditemukan"}`, nil
	}

	out := map[string]any{
		"village_name":   a.VillageName,
		"account_type":   a.AccountType,
		"village_status": deref(a.VillageStatus),
		"territory":      deref(a.Territory),
		"village_budget": maskARR(formatRupiah(a.VillageBudget), canSeeARR(ctx)),
	}
	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(result), nil
}
