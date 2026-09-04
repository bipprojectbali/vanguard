package main

import (
	"context"
	"fmt"

	"go_starter/internal/db"
)

// masters.go — data master lintas-modul yang jadi rujukan domain lain: plans
// (dipakai deals/quotes/subscriptions), sla_policies (tickets), playbooks +
// kb_articles (Customer Success, tak dirujuk FK modul lain — berdiri sendiri).

// masterData = hasil seedMasters, dioper ke domain lain yang butuh plan_id.
type masterData struct {
	plans          []int64 // 4 plan_id, urutan sesuai spec di bawah.
	slaPolicies    []int64 // 3 sla_policy_id: rendah/sedang/tinggi.
	playbookCount  int
	kbArticleCount int
}

// seedMasters membuat 4 plans, 3 sla_policies, 3 playbooks, 6 kb_articles.
//
// plan_category HANYA boleh 'Core'/'Add-on'/'Module' (plans_category_chk,
// migrasi 00011) — 'Enterprise' BUKAN nilai sah, jadi dua plan di sini
// sama-sama 'Core' pada harga berjenjang (Starter vs Enterprise sbg NAMA,
// bukan kategori) supaya tetap dapat 4 baris plan yang variatif harganya.
func seedMasters(ctx context.Context, q *db.Queries, tenantID int64, tag string, owner *int64) (masterData, error) {
	var out masterData

	planSpecs := []struct {
		name, code, category, price, billing, features string
	}{
		{"Paket Inti Starter " + tag, "SEED-CORE-START-" + tag, "Core", "1500000", "Monthly", "Presensi warga, Surat elektronik, Dasbor APBDes"},
		{"Paket Inti Enterprise " + tag, "SEED-CORE-ENT-" + tag, "Core", "4500000", "Monthly", "Semua fitur Starter + multi-user, laporan lanjutan"},
		{"Paket Tambahan Pelaporan " + tag, "SEED-ADD-RPT-" + tag, "Add-on", "500000", "Monthly", "Ekspor laporan PDF/Excel terjadwal"},
		{"Modul SIA Desa " + tag, "SEED-MOD-SIA-" + tag, "Module", "2000000", "Annually", "Sistem Informasi Aset Desa terintegrasi"},
	}
	for _, s := range planSpecs {
		price, err := num(s.price)
		if err != nil {
			return out, err
		}
		p, err := q.CreatePlan(ctx, db.CreatePlanParams{
			TenantID:         tenantID,
			PlanName:         s.name,
			PlanCode:         s.code,
			PlanCategory:     s.category,
			IsActive:         true,
			BasePrice:        price,
			BillingFrequency: &s.billing,
			SetupFee:         mustNum("0"),
			Currency:         "IDR",
			IncludedFeatures: &s.features,
			CreatedBy:        owner,
		})
		if err != nil {
			return out, fmt.Errorf("plan %s: %w", s.code, err)
		}
		out.plans = append(out.plans, p.ID)
	}

	slaSpecs := []struct {
		name, priority       string
		firstMin, resolveMin int32
	}{
		{"SLA Prioritas Tinggi", "Tinggi", 30, 240},
		{"SLA Prioritas Sedang", "Sedang", 120, 720},
		{"SLA Prioritas Rendah", "Rendah", 480, 2880},
	}
	for _, s := range slaSpecs {
		p, err := q.CreateSLAPolicy(ctx, db.CreateSLAPolicyParams{
			TenantID:                   tenantID,
			SlaName:                    s.name,
			AppliesToPriority:          &s.priority,
			FirstResponseTargetMinutes: &s.firstMin,
			ResolutionTargetMinutes:    &s.resolveMin,
			IsActive:                   true,
			CreatedBy:                  owner,
		})
		if err != nil {
			return out, fmt.Errorf("sla %s: %w", s.name, err)
		}
		out.slaPolicies = append(out.slaPolicies, p.ID)
	}

	// trigger/owner WAJIB salah satu dari playbookTriggerScenarioOptions/
	// playbookRecommendedOwnerOptions (internal/handler/playbooks_form.go) —
	// bukan teks bebas, walau migrasi 00017 tak memberi CHECK di DB (validasi
	// disengaja di handler, pola sama sla_policies). 4 spec di bawah menyebar
	// ke seluruh 4 trigger_scenario & seluruh 3 recommended_owner yang sah.
	playbookSpecs := []struct {
		name, trigger, owner, steps string
	}{
		{"Onboarding Desa Baru", "New Onboarding", "CSM",
			"1) Jadwalkan kickoff call. 2) Setup akun & training operator. 3) Verifikasi go-live H+14."},
		{"Penyelamatan Akun At-Risk", "Health Drop", "CSM",
			"1) Hubungi PIC desa. 2) Identifikasi kendala pemakaian. 3) Susun success plan pemulihan."},
		{"Ekspansi Add-on Saat Renewal", "Renewal Approaching", "Sales",
			"1) Review fitur yang belum dipakai. 2) Tawarkan add-on relevan. 3) Buat quote upsell."},
		{"Aktivasi Ulang Fitur Jarang Dipakai", "Low Adoption", "Support",
			"1) Identifikasi fitur yang jarang diakses. 2) Hubungi operator desa. 3) Jadwalkan sesi refresh training."},
	}
	for _, s := range playbookSpecs {
		if _, err := q.CreatePlaybook(ctx, db.CreatePlaybookParams{
			TenantID:         tenantID,
			PlaybookName:     s.name,
			TriggerScenario:  &s.trigger,
			RecommendedOwner: &s.owner,
			Steps:            &s.steps,
			IsActive:         true,
			CreatedBy:        owner,
		}); err != nil {
			return out, fmt.Errorf("playbook %s: %w", s.name, err)
		}
		out.playbookCount++
	}

	// kbSpecs: category & status TERKUNCI enum — category ∈ {Panduan Awal,
	// Pembayaran, Kependudukan, Teknis, Umum} (kb_articles_category_chk, migrasi
	// 00035); status ∈ {Draft, Published, Archived} (kb_articles_status_chk,
	// 00034); visibility ∈ {Public, Internal, Portal Only} (00018). Ubah nilai di
	// sini hanya ke anggota enum sah — TestSeedInto jadi gerbangnya.
	kbSpecs := []struct {
		title, body, category, status, visibility string
	}{
		{"Cara Reset Password Operator", "Panduan reset password akun operator desa lewat menu Pengaturan.", "Teknis", "Published", "Public"},
		{"Panduan Input APBDes", "Langkah input rencana anggaran desa ke modul keuangan.", "Pembayaran", "Published", "Public"},
		{"Troubleshooting Login Gagal", "Daftar penyebab umum login gagal & solusinya.", "Teknis", "Published", "Internal"},
		{"Checklist Onboarding CSM", "Checklist internal CSM saat mengonboarding desa baru.", "Panduan Awal", "Draft", "Internal"},
		{"Draft: Integrasi SIA Desa", "Rancangan dokumentasi integrasi modul SIA (belum final).", "Teknis", "Draft", "Internal"},
		{"FAQ Portal Desa", "Pertanyaan umum warga seputar layanan portal desa.", "Umum", "Published", "Portal Only"},
	}
	for _, s := range kbSpecs {
		if _, err := q.CreateKBArticle(ctx, db.CreateKBArticleParams{
			TenantID:     tenantID,
			ArticleTitle: s.title,
			ArticleBody:  &s.body,
			Category:     &s.category,
			Status:       s.status,
			Visibility:   s.visibility,
			AuthorID:     owner,
			CreatedBy:    owner,
		}); err != nil {
			return out, fmt.Errorf("kb %s: %w", s.title, err)
		}
		out.kbArticleCount++
	}

	return out, nil
}
