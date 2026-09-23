package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// jena_ai_tools_more_test.go — lanjutan jena_ai_tools_test.go
// (dipecah krn ambang File Health; lihat komentar file itu utk konteks).

// --- list_my_deals ------------------------------------------------------------

func TestJenaListMyDeals_F2DeniedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Deal", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid)

	var out string
	env.runJena(uid, "support", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyDeals(ctx)
		if err != nil {
			t.Fatalf("jenaListMyDeals: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data deal") {
		t.Errorf("Support tak punya crm:deals (F2), got %s", out)
	}
}

func TestJenaListMyDeals_F2DeniedForCSM(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Deal CSM", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid)

	var out string
	env.runJena(uid, "csm", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyDeals(ctx)
		if err != nil {
			t.Fatalf("jenaListMyDeals: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data deal") {
		t.Errorf("CSM tak punya crm:deals (BL-11, F2), got %s", out)
	}
}

func TestJenaListMyDeals_ForcesOwnOnlyEvenForManager(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-manager@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Deal Saya", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Deal Orang Lain", &other, nil, nil)
	env.seedDeal(t, accMine.ID, &uid)
	env.seedDeal(t, accOther.ID, &other)

	// Manager punya data_scope=all (ScopeAll) — tanpa pemaksaan IsOwn, ia akan
	// melihat SEMUA deal. list_my_deals harus tetap hanya deal milik uid.
	var out string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyDeals(ctx)
		if err != nil {
			t.Fatalf("jenaListMyDeals: %v", err)
		}
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("Manager (ScopeAll) harus tetap hanya melihat deal MILIK SENDIRI lewat Jena AI, got %d baris: %s", len(results), out)
	}
}

// --- list_my_leads ------------------------------------------------------------

func TestJenaListMyLeads_F2DeniedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLead(t, "Lead Uji", &uid)

	var out string
	env.runJena(uid, "support", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyLeads(ctx)
		if err != nil {
			t.Fatalf("jenaListMyLeads: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data lead") {
		t.Errorf("Support tak punya crm:leads (F2), got %s", out)
	}
}

func TestJenaListMyLeads_F2DeniedForCSM(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLead(t, "Lead Uji CSM", &uid)

	var out string
	env.runJena(uid, "csm", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyLeads(ctx)
		if err != nil {
			t.Fatalf("jenaListMyLeads: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data lead") {
		t.Errorf("CSM tak punya crm:leads (BL-11, F2), got %s", out)
	}
}

func TestJenaListMyLeads_ForcesOwnOnlyEvenForManager(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-manager-lead@local", "member", 0).ID
	env.seedLead(t, "Lead Milik Saya", &uid)
	env.seedLead(t, "Lead Milik Orang Lain", &other)

	// Manager punya data_scope=all (ScopeAll) — tanpa pemaksaan IsOwn, ia akan
	// melihat SEMUA lead. list_my_leads harus tetap hanya lead milik uid.
	var out string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyLeads(ctx)
		if err != nil {
			t.Fatalf("jenaListMyLeads: %v", err)
		}
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("Manager (ScopeAll) harus tetap hanya melihat lead MILIK SENDIRI lewat Jena AI, got %d baris: %s", len(results), out)
	}
}

// --- dispatcher: audit tanpa isi -----------------------------------------------

func TestJenaDispatch_AuditNoContent(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Rahasia", &uid, nil, nil)

	env.runJena(uid, "sales", func(ctx context.Context) {
		if _, err := env.h.jenaDispatch(ctx, "get_account_summary", json.RawMessage(itoaJSON("account_id", acc.ID))); err != nil {
			t.Fatalf("jenaDispatch: %v", err)
		}
	})

	logs, err := env.q.ListAuditLogs(t.Context(), 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("aksi harus tercatat di audit_logs (err=%v)", err)
	}
	found := false
	for _, l := range logs {
		if l.Action == "jena_ai.tool_call" {
			found = true
			meta := string(l.Metadata)
			if strings.Contains(meta, "Desa Rahasia") {
				t.Errorf("audit TAK BOLEH memuat isi hasil tool (gotcha #12/#15), got meta=%s", meta)
			}
			if !strings.Contains(meta, "get_account_summary") {
				t.Errorf("audit harus memuat nama tool, got meta=%s", meta)
			}
		}
	}
	if !found {
		t.Error("audit log jena_ai.tool_call tidak ditemukan")
	}
}

func TestJenaDispatch_UnknownTool(t *testing.T) {
	env, uid := setupAccounts(t)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaDispatch(ctx, "delete_everything", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("jenaDispatch: %v", err)
		}
	})
	if !strings.Contains(out, "tool tidak dikenal") {
		t.Errorf("tool di luar allowlist harus balas error defensif, got %s", out)
	}
}

// itoaJSON membangun payload JSON satu field integer — helper kecil biar test
// di atas tak mengulang fmt.Sprintf.
func itoaJSON(field string, id int64) string {
	b, _ := json.Marshal(map[string]int64{field: id})
	return string(b)
}
