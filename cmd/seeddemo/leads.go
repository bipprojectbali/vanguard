package main

import (
	"context"
	"fmt"
	"math/rand"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// leads.go — 25 leads across 5 status (8 New/6 Contacted/5 Qualified/
// 3 Unqualified/3 Converted). Tiap lead mengambil satu Desa REAL dari pool
// (BL-66): lead_name = nama Desa asli, district_id = kecamatan induk. 3 Converted
// MENIRU alur nyata internal/handler/sales_convert_action.go (buat account+
// contact+deal lalu MarkLeadConverted) dan account hasilnya MEMAKAI Desa yang
// sama (nama + village_code Kemendagri asli), bukan cuma set kolom status.

var leadSources = []string{"Website", "Referral", "Cold Call", "Event", "Pemda", "Media Sosial"}
var unqualifiedReasons = []string{"Anggaran tidak cukup", "Sudah pakai kompetitor", "Tidak responsif", "Bukan target segmen"}

// leadResult = ringkasan hasil seedLeads: total baris + account BARU yang
// tercipta dari konversi (perlu ditambahkan ke daftar account utk domain
// hilir — deals/quotes/subscriptions/dst).
type leadResult struct {
	total             int
	convertedAccounts []accountInfo
}

func seedLeads(ctx context.Context, q *db.Queries, tenantID int64, tag string, rng *rand.Rand, owner *int64, pool *villagePool) (leadResult, error) {
	var out leadResult

	specs := []struct {
		status string
		count  int
	}{
		{"New", 8}, {"Contacted", 6}, {"Qualified", 5}, {"Unqualified", 3}, {"Converted", 3},
	}

	seq := 0
	for _, spec := range specs {
		for i := 0; i < spec.count; i++ {
			seq++
			desa, err := pool.next()
			if err != nil {
				return out, err
			}
			code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityLead)
			if err != nil {
				return out, fmt.Errorf("kode lead: %w", err)
			}
			did := desa.DistrictID
			contact := pick(rng, firstNames) + " " + pick(rng, lastNames)
			mobile := fmt.Sprintf("0813%08d", rng.Intn(100000000))
			email := fmt.Sprintf("lead%d.%s@contoh.id", seq, tag)
			source := pick(rng, leadSources)
			estVal, err := num(fmt.Sprintf("%d.00", 3_000_000+rng.Intn(20_000_000)))
			if err != nil {
				return out, err
			}

			var rating *string
			var unqualified *string
			switch spec.status {
			case "Qualified", "Converted":
				rating = ptr("Hot")
			case "Contacted":
				rating = ptr(pick(rng, []string{"Warm", "Cold"}))
			case "Unqualified":
				unqualified = ptr(pick(rng, unqualifiedReasons))
			}

			var leadOwner *int64
			if rng.Intn(100) < 70 {
				leadOwner = owner
			}

			lead, err := q.CreateLead(ctx, db.CreateLeadParams{
				TenantID:          tenantID,
				EntityCode:        &code,
				LeadName:          desa.Name, // BL-66: nama Desa REAL, sama dgn account bila dikonversi
				LeadOwner:         leadOwner,
				ContactPerson:     &contact,
				JobTitle:          ptr(pick(rng, positionCategories)),
				LeadSource:        &source,
				LeadStatus:        spec.status,
				Rating:            rating,
				UnqualifiedReason: unqualified,
				EstimatedValue:    estVal,
				DistrictID:        &did,
				MobilePhone:       &mobile,
				Whatsapp:          &mobile,
				Email:             &email,
				CreatedBy:         owner,
			})
			if err != nil {
				return out, fmt.Errorf("lead #%d: %w", seq, err)
			}
			out.total++

			if spec.status != "Converted" {
				continue
			}
			acc, err := convertLead(ctx, q, tenantID, owner, lead, contact, desa)
			if err != nil {
				return out, fmt.Errorf("konversi lead #%d: %w", lead.ID, err)
			}
			out.convertedAccounts = append(out.convertedAccounts, acc)
		}
	}
	return out, nil
}

// convertLead meniru alur sales_convert_action.go: Account → Contact → Deal
// (stage awal "Prospecting") → MarkLeadConverted menautkan ketiganya ke lead.
// BL-66: account hasil konversi memakai Desa REAL yang sama dgn lead (nama +
// village_code Kemendagri asli + kecamatan induk).
func convertLead(ctx context.Context, q *db.Queries, tenantID int64, owner *int64, lead db.Lead, contactName string, desa village) (accountInfo, error) {
	accCode, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		return accountInfo{}, fmt.Errorf("kode account: %w", err)
	}
	villageCode := desa.Code
	districtID := desa.DistrictID
	acc, err := q.CreateAccount(ctx, db.CreateAccountParams{
		TenantID:      tenantID,
		EntityCode:    &accCode,
		VillageName:   desa.Name,
		VillageCode:   &villageCode,
		AccountType:   "customer",
		AccountOwner:  owner,
		AssignedCsm:   owner,
		DistrictID:    &districtID,
		VillageStatus: ptr("Desa"),
		CreatedBy:     owner,
	})
	if err != nil {
		return accountInfo{}, fmt.Errorf("account hasil konversi: %w", err)
	}

	parts := splitName(contactName)
	con, err := q.CreateContact(ctx, db.CreateContactParams{
		TenantID:         tenantID,
		AccountID:        acc.ID,
		ContactOwner:     owner,
		FirstName:        parts[0],
		LastName:         &parts[1],
		JobTitle:         lead.JobTitle,
		IsPrimaryContact: true,
		MobilePhone:      lead.MobilePhone,
		WhatsappNumber:   lead.Whatsapp,
		Email:            lead.Email,
	})
	if err != nil {
		return accountInfo{}, fmt.Errorf("contact hasil konversi: %w", err)
	}

	dealCode, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
	if err != nil {
		return accountInfo{}, fmt.Errorf("kode deal: %w", err)
	}
	deal, err := q.CreateDeal(ctx, db.CreateDealParams{
		TenantID:         tenantID,
		EntityCode:       &dealCode,
		DealName:         lead.LeadName + " — Langganan Awal",
		AccountID:        acc.ID,
		DealOwner:        owner,
		PrimaryContactID: &con.ID,
		Stage:            "Prospecting",
		Amount:           lead.EstimatedValue,
		CreatedBy:        owner,
	})
	if err != nil {
		return accountInfo{}, fmt.Errorf("deal hasil konversi: %w", err)
	}

	if err := q.MarkLeadConverted(ctx, db.MarkLeadConvertedParams{
		ConvertedAccountID: &acc.ID,
		ConvertedContactID: &con.ID,
		ConvertedDealID:    &deal.ID,
		UpdatedBy:          owner,
		ID:                 lead.ID,
	}); err != nil {
		return accountInfo{}, fmt.Errorf("mark converted: %w", err)
	}

	return accountInfo{ID: acc.ID, Type: "customer"}, nil
}

// splitName memecah "Depan Belakang" jadi [depan, belakang] — fallback nama
// depan dua kali bila cuma satu kata (jarang, tapi jangan panic).
func splitName(full string) [2]string {
	for i, r := range full {
		if r == ' ' {
			return [2]string{full[:i], full[i+1:]}
		}
	}
	return [2]string{full, full}
}
