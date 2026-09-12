package main

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/fls"

	"github.com/jackc/pgx/v5/pgxpool"
)

// wiring.go — wiring authz (Casbin) yang dipanggil dari run(); dipisah agar
// run.go tetap ringkas (§file-health).

// wireAuthz memuat & meng-init enforcer Casbin inti, enforcer sumbu bisnis CRM,
// dan kebijakan FLS phone — dipanggil sekali saat boot, sebelum handler dirakit.
func wireAuthz(ctx context.Context, pool *pgxpool.Pool) error {
	// Authz (Casbin) — enforcer in-memory dari model+policy embed.
	enforcer, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		return err
	}
	authz.Init(enforcer)

	// Sumbu BISNIS (CRM) — enforcer TERPISAH atas business_role. Sengaja bukan
	// menumpang enforcer di atas: god-mode root & warisan platform→tenant di sana
	// akan membocorkan izin CRM ke owner/super_admin (§3).
	//
	// SEJAK 00007 peran bisa diedit per-workspace → policy dibaca dari DB, bukan
	// CSV embed. Subject di-fold `t<id>:<role>` di authz agar peran senama di dua
	// workspace tak saling memberi izin. Load-nya WAJIB WithSuper: di tenant-tx RLS
	// menyembunyikan tenant lain → enforcer deny-all senyap bagi mereka.
	bEnforcer, err := authz.NewBusinessEmpty()
	if err != nil {
		return err
	}
	perms, err := loadBusinessPerms(ctx, pool)
	if err != nil {
		return err
	}
	if err := authz.LoadBusiness(bEnforcer, perms); err != nil {
		return err
	}
	authz.InitBusiness(bEnforcer)

	// FLS phone (BL-107): kebijakan lihat/sunting HP & WhatsApp konfigurabel per-tenant,
	// di-cache in-proses (pola enforcer bisnis di atas). Load-all WithSuper; tenant tanpa
	// baris jatuh ke default terkunci (Sales+Admin lihat, Sales sunting).
	fsPolicies, err := loadFieldSecurity(ctx, pool)
	if err != nil {
		return err
	}
	fls.Load(fsPolicies)
	return nil
}
