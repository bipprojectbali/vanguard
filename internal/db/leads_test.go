package db

import (
	"context"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgxpool"
)

// leads_test.go — bukti query funnel Lead (M4). Empat sifat yang, bila rusak, tak
// terlihat dari perilaku aplikasi sampai data sudah bocor/hilang:
//
//   (1) Roundtrip create→get: entity_code (LEAD-001) dialokasikan DI DALAM tx dan
//       ikut tersimpan; status default 'New'.
//   (2) ListLeads = keyset + ownership F3 dalam satu query. Keyset harus bisa
//       menjangkau halaman kedua (bukan cuma LIMIT); ownership harus fail-closed
//       (flag semua false → nol baris, bukan bocor).
//   (3) MarkLeadConverted hanya menyentuh lead Qualified & belum converted — guard
//       di WHERE, bukan sekadar disiplin handler (konversi ganda idempotent-aman).
//   (4) CHECK lead_status menolak nilai di luar enum; soft-delete menyembunyikan.
//
// Seed lewat CreateLead asli (jalur yang diuji), di dalam WithTenant (RLS + SET
// LOCAL ROLE app_rw aktif — persis jalur query aplikasi).

// seedLead membuat satu lead lewat CreateLead asli (dengan entity_code dari
// GenerateEntityCode) di dalam tx ber-tenant. Mengembalikan baris hasil.
func seedLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID int64, name string, mut func(*CreateLeadParams)) Lead {
	t.Helper()
	var out Lead
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityLead)
		if err != nil {
			return err
		}
		p := CreateLeadParams{
			TenantID:   tenantID,
			EntityCode: &code,
			LeadName:   name,
			LeadStatus: "New",
		}
		if mut != nil {
			mut(&p)
		}
		out, err = q.CreateLead(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed lead %q: %v", name, err)
	}
	return out
}

func TestCreateAndGetLead(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}

	person := "Budi"
	lead := seedLead(t, ctx, pool, ten.ID, "Sukamaju", func(p *CreateLeadParams) {
		p.ContactPerson = &person
	})

	if lead.EntityCode == nil || *lead.EntityCode != "LEAD-001" {
		t.Errorf("entity_code pertama harus LEAD-001, got %v", lead.EntityCode)
	}
	if lead.LeadStatus != "New" {
		t.Errorf("status default harus New, got %q", lead.LeadStatus)
	}

	var got Lead
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetLead(ctx, lead.ID)
		return e
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != lead.ID || got.LeadName != "Sukamaju" {
		t.Errorf("get harus kembalikan lead yang sama, got id=%d name=%q", got.ID, got.LeadName)
	}
	if got.ContactPerson == nil || *got.ContactPerson != person {
		t.Errorf("contact_person harus tersimpan apa adanya, got %v", got.ContactPerson)
	}
}

func TestListLeads_KeysetPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	for _, n := range []string{"Satu", "Dua", "Tiga"} {
		seedLead(t, ctx, pool, ten.ID, n, nil)
	}

	// Halaman 1: page_size=2 → 2 baris terbaru (Tiga, Dua) karena DESC.
	c0, id0 := firstCursor()
	var p1 []Lead
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p1, e = q.ListLeads(ctx, ListLeadsParams{CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 2})
		return e
	}); err != nil {
		t.Fatalf("list p1: %v", err)
	}
	if len(p1) != 2 {
		t.Fatalf("halaman 1 harus 2 baris, got %d", len(p1))
	}
	if p1[0].LeadName != "Tiga" || p1[1].LeadName != "Dua" {
		t.Errorf("urutan DESC salah: %q,%q", p1[0].LeadName, p1[1].LeadName)
	}

	// Halaman 2: cursor dari baris terakhir halaman 1 → sisa 1 baris (Satu).
	last := p1[1]
	var p2 []Lead
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p2, e = q.ListLeads(ctx, ListLeadsParams{CursorCreatedAt: last.CreatedAt, CursorID: last.ID, ScopeAll: true, PageSize: 2})
		return e
	}); err != nil {
		t.Fatalf("list p2: %v", err)
	}
	if len(p2) != 1 || p2[0].LeadName != "Satu" {
		t.Fatalf("halaman 2 harus 1 baris (Satu), got %d", len(p2))
	}
}

func TestListLeads_OwnershipFailClosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	seedLead(t, ctx, pool, ten.ID, "Milik-Sales", func(p *CreateLeadParams) { p.LeadOwner = &sales.ID })
	seedLead(t, ctx, pool, ten.ID, "Milik-Lain", func(p *CreateLeadParams) { p.LeadOwner = &other.ID })

	listWith := func(f LeadsListFilter, uid int64) []Lead {
		var rows []Lead
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListLeads(ctx, ListLeadsParams{
				CursorCreatedAt: c0, CursorID: id0,
				ScopeAll: f.ScopeAll, IsOwn: f.IsOwn, Uid: &uid,
				PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	// Admin/Manager (ScopeAll): 2 lead.
	if got := listWith(LeadsListFilter{ScopeAll: true}, sales.ID); len(got) != 2 {
		t.Errorf("scope_all harus 2 lead, got %d", len(got))
	}
	// Sales (IsOwn): hanya lead_owner-nya = 1.
	if got := listWith(LeadsListFilter{IsOwn: true}, sales.ID); len(got) != 1 || got[0].LeadName != "Milik-Sales" {
		t.Errorf("own/sales harus 1 (Milik-Sales), got %d", len(got))
	}
	// Semua flag false (none/role liar): NOL baris — fail-closed, bukan bocor.
	// Inilah kriteria penerimaan F3: "query TANPA filter ini = test gagal".
	if got := listWith(LeadsListFilter{}, sales.ID); len(got) != 0 {
		t.Errorf("tanpa flag harus 0 baris (fail-closed), got %d", len(got))
	}
}

func TestListLeads_StatusFilterAndMineOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	unq := "Duplicate"
	seedLead(t, ctx, pool, ten.ID, "Baru-Ku", func(p *CreateLeadParams) { p.LeadOwner = &sales.ID })
	seedLead(t, ctx, pool, ten.ID, "Ditolak", func(p *CreateLeadParams) {
		p.LeadOwner = &sales.ID
		p.LeadStatus = "Unqualified"
		p.UnqualifiedReason = &unq
	})
	seedLead(t, ctx, pool, ten.ID, "Punya-Lain", func(p *CreateLeadParams) { p.LeadOwner = &other.ID })

	// status_filter 'Unqualified' → hanya lead ditolak (aktor scope_all).
	var byStatus []Lead
	c0, id0 := firstCursor()
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		byStatus, e = q.ListLeads(ctx, ListLeadsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true,
			StatusFilter: "Unqualified", PageSize: 50,
		})
		return e
	}); err != nil {
		t.Fatalf("list status: %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].LeadName != "Ditolak" {
		t.Errorf("status_filter Unqualified harus 1 (Ditolak), got %d", len(byStatus))
	}

	// mine_only → paksa lead_owner = uid walau aktor scope_all (tab "My Leads").
	var mine []Lead
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		mine, e = q.ListLeads(ctx, ListLeadsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true,
			Uid: &sales.ID, MineOnly: true, PageSize: 50,
		})
		return e
	}); err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(mine) != 2 {
		t.Errorf("mine_only sales harus 2 (miliknya saja), got %d", len(mine))
	}
}

func TestMarkLeadConverted_HanyaQualified(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	// Lead masih 'New' → guard di WHERE menahan; tak jadi Converted.
	fresh := seedLead(t, ctx, pool, ten.ID, "MasihBaru", nil)
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.MarkLeadConverted(ctx, MarkLeadConvertedParams{ID: fresh.ID})
	}); err != nil {
		t.Fatalf("mark (new): %v", err)
	}
	got := mustGetLead(t, ctx, pool, ten.ID, fresh.ID)
	if got.Converted || got.LeadStatus == "Converted" {
		t.Errorf("lead New tak boleh jadi converted (guard WHERE), got converted=%v status=%q", got.Converted, got.LeadStatus)
	}

	// Lead 'Qualified' → boleh; status jadi Converted + converted=true.
	ready := seedLead(t, ctx, pool, ten.ID, "SiapKonversi", func(p *CreateLeadParams) { p.LeadStatus = "Qualified" })
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.MarkLeadConverted(ctx, MarkLeadConvertedParams{ID: ready.ID})
	}); err != nil {
		t.Fatalf("mark (qualified): %v", err)
	}
	got = mustGetLead(t, ctx, pool, ten.ID, ready.ID)
	if !got.Converted || got.LeadStatus != "Converted" {
		t.Errorf("lead Qualified harus jadi Converted, got converted=%v status=%q", got.Converted, got.LeadStatus)
	}

	// Konversi kedua atas lead yang sudah Converted → idempotent-aman (NOT converted
	// gagal, baris tak berubah).
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.MarkLeadConverted(ctx, MarkLeadConvertedParams{ID: ready.ID})
	}); err != nil {
		t.Errorf("mark kedua harus idempotent, got %v", err)
	}
}

func TestSoftDeleteLead(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	lead := seedLead(t, ctx, pool, ten.ID, "Buang", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteLead(ctx, SoftDeleteLeadParams{ID: lead.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	// Get harus gagal (baris disembunyikan filter deleted_at IS NULL).
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetLead(ctx, lead.ID)
		return e
	})
	if err == nil {
		t.Errorf("get atas lead ter-soft-delete harus gagal (tak ada baris hidup)")
	}

	// Idempotent: soft-delete kedua tak error.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteLead(ctx, SoftDeleteLeadParams{ID: lead.ID})
	}); e != nil {
		t.Errorf("soft-delete kedua harus idempotent, got %v", e)
	}
}

func TestCreateLead_StatusCheckMenolakLiar(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	// lead_status di luar enum → ditolak leads_status_chk.
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		code, e := q.GenerateEntityCode(ctx, ten.ID, codes.EntityLead)
		if e != nil {
			return e
		}
		_, e = q.CreateLead(ctx, CreateLeadParams{
			TenantID: ten.ID, EntityCode: &code, LeadName: "Liar", LeadStatus: "Galaxy",
		})
		return e
	})
	if err == nil {
		t.Errorf("lead_status liar harus ditolak CHECK constraint")
	}
}

// ── Unit (tanpa DB) — perakitan flag & keputusan per-baris ──────────────────

// TestLeadsListFilterFor: data_scope → flag list-query. Sumber SATU dengan
// AccountsScopeFor; fail-closed untuk nilai liar.
func TestLeadsListFilterFor(t *testing.T) {
	cases := []struct {
		scope string
		want  LeadsListFilter
	}{
		{authz.DataScopeAll, LeadsListFilter{ScopeAll: true}},
		{authz.DataScopeOwn, LeadsListFilter{IsOwn: true}},
		{authz.DataScopeNone, LeadsListFilter{}},
		{"", LeadsListFilter{}},
		{"galaxy", LeadsListFilter{}},
	}
	for _, c := range cases {
		if got := LeadsListFilterFor(c.scope); got != c.want {
			t.Errorf("LeadsListFilterFor(%q)=%+v, want %+v", c.scope, got, c.want)
		}
	}
}

// TestLeadsListFilter_Allows: keputusan per-baris (dipakai handler untuk detail
// GetLead) konsisten dengan yang disaring ListLeads.
func TestLeadsListFilter_Allows(t *testing.T) {
	const me, notMe = int64(7), int64(99)
	p := func(v int64) *int64 { return &v }

	if !(LeadsListFilter{ScopeAll: true}).Allows(me, nil) {
		t.Error("ScopeAll harus mengizinkan baris tanpa owner")
	}
	own := LeadsListFilter{IsOwn: true}
	if !own.Allows(me, p(me)) {
		t.Error("own harus boleh atas lead miliknya")
	}
	if own.Allows(me, p(notMe)) {
		t.Error("own tak boleh atas lead orang lain")
	}
	if own.Allows(me, nil) {
		t.Error("own tak boleh atas lead tanpa owner")
	}
	if (LeadsListFilter{}).Allows(me, p(me)) {
		t.Error("filter kosong harus menolak semua (fail-closed)")
	}
}

// getLead = GetLead ringkas untuk test.
func mustGetLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, id int64) Lead {
	t.Helper()
	var got Lead
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		got, e = q.GetLead(ctx, id)
		return e
	}); err != nil {
		t.Fatalf("get lead %d: %v", id, err)
	}
	return got
}
