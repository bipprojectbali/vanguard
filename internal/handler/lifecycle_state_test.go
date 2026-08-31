package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// lifecycle_state_test.go — transisi status & siklus hidup (soft-delete, arsip,
// purge, restore, kuota) dari lifecycle_test.go, dipisah agar tiap file di bawah
// ambang tipe Test (400). Helper setStatus/gate & gerbang tetap di
// lifecycle_test.go (paket sama).

// TestSoftDelete_HilangDariSwitcherDanKuota: workspace terhapus tak boleh muncul
// di switcher (0005) — ia juga sumber pilihan fallback Scope, jadi kalau tampak,
// user bisa dilempar ke workspace yang sudah dihapus. Kuota pun tak boleh
// tertahan olehnya: itu terasa seperti bug & mendorong purge lebih cepat.
func TestSoftDelete_HilangDariSwitcherDanKuota(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	before, _ := env.q.CountOwnedWorkspaces(ctx, uid)
	if before != 1 {
		t.Fatalf("awal harus 1 workspace dimiliki, got %d", before)
	}
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	ms, err := env.q.ListMembershipsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(ms) != 0 {
		t.Errorf("workspace terhapus masih muncul di switcher: %+v", ms)
	}
	after, _ := env.q.CountOwnedWorkspaces(ctx, uid)
	if after != 0 {
		t.Errorf("kuota masih tertahan workspace terhapus: %d", after)
	}
}

// TestArchived_TetapMemakanKuota: kebalikan dari terhapus — datanya masih
// disimpan & bisa diaktifkan kapan saja, jadi arsip tak boleh jadi celah kuota
// gratis (0005 §7).
func TestArchived_TetapMemakanKuota(t *testing.T) {
	env, uid := setupTest(t)
	setStatus(t, env, env.tenantID, TenantArchived)

	n, err := env.q.CountOwnedWorkspaces(t.Context(), uid)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("workspace terarsip HARUS tetap memakan kuota, got %d", n)
	}
}

// TestRestore_MengembalikanKeadaanTerpakai: pemulihan harus meninggalkan
// workspace yang bisa langsung dipakai, bukan setengah jalan (mis. kembali
// sebagai terarsip sehingga owner harus menekan tombol kedua).
func TestRestore_MengembalikanKeadaanTerpakai(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	setStatus(t, env, env.tenantID, TenantArchived)
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := env.q.RestoreTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	tn, _ := env.q.GetTenant(ctx, env.tenantID)
	if tn.DeletedAt.Valid {
		t.Error("restore harus membatalkan penghapusan")
	}
	if tn.Status != TenantActive {
		t.Errorf("status setelah restore = %q, want active (siap pakai)", tn.Status)
	}
	ms, _ := env.q.ListMembershipsByUser(ctx, uid)
	if len(ms) != 1 {
		t.Errorf("workspace harus kembali ke switcher, got %d", len(ms))
	}
}

// TestPurge_AuditSelamat: BUKTI TAK BOLEH LENYAP BERSAMA YANG DIBUKTIKAN.
// memberships ikut CASCADE (data operasional), audit_logs TIDAK — FK-nya
// ON DELETE SET NULL sejak migrasi 00010. Justru pada peristiwa terpenting
// (penghapusan workspace) jejaknya paling dibutuhkan.
func TestPurge_AuditSelamat(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	if _, err := env.q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: &uid, Action: "workspace.delete", TargetType: "tenant",
		TargetID: &env.tenantID, Metadata: []byte("{}"), TenantID: &env.tenantID,
	}); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := env.q.PurgeTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("purge GAGAL — FK audit memblokir penghapusan: %v", err)
	}
	// Tenant benar-benar hilang…
	if _, err := env.q.GetTenant(ctx, env.tenantID); err == nil {
		t.Error("tenant harus terhapus permanen setelah purge")
	}
	// …tapi jejaknya tidak.
	var n int64
	if err := env.h.Pool.QueryRow(ctx,
		"SELECT count(*) FROM audit_logs WHERE action = 'workspace.delete'").Scan(&n); err != nil {
		t.Fatalf("hitung audit: %v", err)
	}
	if n != 1 {
		t.Errorf("audit log harus SELAMAT dari purge, got %d baris", n)
	}
}

// TestListExpired_HanyaYangLewatTenggang: purge hanya menyentuh yang sudah
// melewati masa tenggang. Kalau tidak, "soft delete" cuma jeda semu.
func TestListExpired_HanyaYangLewatTenggang(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	cutoff := time.Now().Add(-time.Duration(GracePeriodDays) * 24 * time.Hour)

	rows, err := env.q.ListExpiredTenants(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("workspace yang baru dihapus BELUM boleh dipurge, got %d", len(rows))
	}
	// Yang dihapus jauh di masa lalu → kandidat sah.
	if _, err := env.h.Pool.Exec(ctx,
		"UPDATE tenants SET deleted_at = now() - interval '60 days' WHERE id = $1", env.tenantID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	rows, err = env.q.ListExpiredTenants(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		t.Fatalf("list expired (2): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("workspace lewat tenggang harus jadi kandidat purge, got %d", len(rows))
	}
}

// TestSlugTakDilepasSaatTerhapus: slug tetap dipesan selama masa tenggang —
// kalau dilepas, orang lain bisa mengambilnya dan restore jadi mustahil.
func TestSlugTakDilepasSaatTerhapus(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	taken, err := env.q.TenantSlugExists(ctx, "test")
	if err != nil {
		t.Fatalf("slug exists: %v", err)
	}
	if !taken {
		t.Error("slug workspace terhapus HARUS tetap terpesan (restore jadi mustahil bila diambil orang)")
	}
}

// TestScopeSlug_TerhapusDitolak: gerbang di Scope, bukan cuma di query. Anggota
// yang membuka slug workspace terhapus tak boleh menembus ke dalamnya.
func TestScopeSlug_TerhapusDitolak(t *testing.T) {
	env, uid := setupTest(t)
	if err := env.q.SoftDeleteTenant(t.Context(), env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	env.withSession(t, uid, func(sc sessionCtx) {
		// resolveTenantBySlug tetap MENEMUKAN barisnya (jalur restore platform
		// membutuhkannya) — yang menolak adalah gateLifecycle.
		tn, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "test")
		if !ok {
			t.Fatal("baris tenant harus tetap terbaca untuk jalur restore")
		}
		if !tn.DeletedAt.Valid {
			t.Fatal("tenant harus bertanda terhapus")
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/w/test", nil).WithContext(sc.ctx)
		if env.h.gateLifecycle(rec, req, tn) {
			t.Error("workspace terhapus tak boleh lolos gerbang")
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}
