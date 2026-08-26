package main

import (
	"context"
	"fmt"
	"math/rand"

	"go_starter/internal/db"
)

// contacts.go — PIC desa (Contact). 2-3 kontak per desa customer/prospect
// (±90 total), 1 utk former_customer; TEPAT SATU is_primary_contact=true per
// desa (idx_contacts_primary partial unique) — kontak pertama tiap desa yang
// ditandai primary, sisanya tidak.

var firstNames = []string{
	"Budi", "Siti", "Agus", "Dewi", "Ahmad", "Rina", "Hendra", "Fitri",
	"Joko", "Wulan", "Bambang", "Sri", "Eko", "Yuni", "Wawan", "Ratna",
	"Dedi", "Lestari", "Sutrisno", "Indah", "Slamet", "Nur", "Iwan", "Ani",
}

var lastNames = []string{
	"Santoso", "Wijaya", "Kurniawan", "Saputra", "Hidayat", "Setiawan",
	"Pratama", "Gunawan", "Nugroho", "Susanto", "Firmansyah", "Ramadhan",
}

var positionCategories = []string{
	"Kepala Desa", "Sekdes", "Kaur", "Kasi", "Operator", "Bendahara", "BPD", "Lainnya",
}
var contactRoles = []string{"Decision Maker", "Influencer", "User", "Finance", "Gatekeeper"}
var channels = []string{"WhatsApp", "Telepon", "Email", "Kunjungan"}

// seedContacts mengembalikan ID tiap kontak yang dibuat (dipakai activities.go
// sbg target_id nyata utk target_type='contact').
func seedContacts(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) ([]int64, error) {
	var ids []int64
	for _, a := range accounts {
		n := 2 + rng.Intn(2) // 2-3
		if a.Type == "former_customer" {
			n = 1
		}
		for c := 0; c < n; c++ {
			first := pick(rng, firstNames)
			last := pick(rng, lastNames)
			mobile := fmt.Sprintf("0812%08d", rng.Intn(100000000))
			wa := fmt.Sprintf("0812%08d", rng.Intn(100000000))
			email := fmt.Sprintf("%s.%s@contoh-desa.id", toLowerASCII(first), toLowerASCII(last))
			jobTitle := pick(rng, positionCategories)

			var contactOwner *int64
			if rng.Intn(100) < 70 {
				contactOwner = owner
			}

			con, err := q.CreateContact(ctx, db.CreateContactParams{
				TenantID:         tenantID,
				AccountID:        a.ID,
				ContactOwner:     contactOwner,
				FirstName:        first,
				LastName:         &last,
				JobTitle:         &jobTitle,
				PositionCategory: ptr(pick(rng, positionCategories)),
				ContactRole:      ptr(pick(rng, contactRoles)),
				IsPrimaryContact: c == 0,
				MobilePhone:      &mobile,
				WhatsappNumber:   &wa,
				Email:            &email,
				PreferredChannel: ptr(pick(rng, channels)),
			})
			if err != nil {
				return ids, fmt.Errorf("kontak akun #%d: %w", a.ID, err)
			}
			ids = append(ids, con.ID)
		}
	}
	return ids, nil
}

// toLowerASCII lowercase sederhana tanpa import "strings" ekstra di sini
// (dipakai berulang bareng villageSlug — cukup ASCII, nama sudah Latin).
func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
