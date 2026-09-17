package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_cs.go — grup nav Customer Success (wireframe 6, Modul 6).
// Sama pola dengan Settings (workspace_nav_settings.go): grup disembunyikan
// TOTAL bila user tak berhak ke satu pun anaknya (grup kosong tak menawarkan
// apa pun); anak yang tersisa enabled mengikuti izin gerbang halamannya
// masing-masing ("nol menu hantu").

// workspaceCSGroup merakit grup Customer Success (wireframe 6). canSLA =
// canViewSLAPolicies (izin SAMA dengan gerbang SLAPoliciesList). canPlaybooks
// = canViewPlaybooks. canKB = canViewKBArticles. canTickets = canViewTickets
// (izin SAMA dengan gerbang TicketsList, objek crm:tickets read).
// canHealthScore = canViewHealthScore (izin SAMA dengan gerbang HealthScoreList,
// objek crm:health read). canSuccessPlans = canViewSuccessPlans (crm:success_plans read).
// canEngagements = canViewEngagements (izin SAMA dengan gerbang EngagementsList,
// objek crm:engagements read). canRenewals = canViewCSRenewals (crm:renewal_mgmt read).
// canJourney = canReadCSJourney (Customer Journey / Lifecycle 6.2, crm:journey read).
//
// Implementation Tracker (6.2.1.1) SENGAJA tak punya item nav (BL-78, level A
// — hanya disembunyikan dari menu; entry-link tambahan di halaman Customer
// Success juga dipause, BL-167). Route, handler, tabel & gate crm:journey
// (canViewImplTasks) tetap hidup; item mudah dikembalikan dengan menambah
// kembali NavItem-nya.
//
// Training Schedule (6.2.1.2) DIKEMBALIKAN ke menu (BL-168, 17 Sep — keputusan
// user: hanya Training Schedule yang ditampilkan lagi, Implementation Tracker
// TETAP dipause). canTrainings = canViewTrainings — SEJAK BL-169 objek Casbin
// SENDIRI (`crm:adoption`, direlabel "Training Schedule" di matriks /roles),
// TERPISAH dari canJourney (dulu numpang satu objek `crm:journey` yang sama
// dengan Customer Journey — admin tak bisa mengatur akses keduanya terpisah).
func workspaceCSGroup(slug string, canSLA, canPlaybooks, canKB, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals, canJourney, canTrainings bool) *ui.NavItem {
	if !canSLA && !canPlaybooks && !canKB && !canTickets && !canHealthScore &&
		!canSuccessPlans && !canEngagements && !canRenewals && !canJourney && !canTrainings {
		return nil
	}
	children := make([]ui.NavItem, 0, 12)
	// Health Score → /health-scores (canViewHealthScore, objek crm:health read).
	// Berbackend sejak slice C1; tanpa izin → tak ditampilkan sama sekali.
	if canHealthScore {
		children = append(children, ui.NavItem{
			Label: "Health Score", Href: wsPath(slug, "/health-scores"),
			Icon: lucide.HeartPulse(html.Class("size-4")),
		})
	}
	// Customer Journey → /journey (canReadCSJourney, crm:journey read — objek
	// SAMA dgn Implementation Tracker/Training). Dashboard portofolio fase
	// lifecycle desa (6.2, BL-77); tanpa izin → tak ditampilkan.
	if canJourney {
		children = append(children, ui.NavItem{
			Label: "Customer Journey", Href: wsPath(slug, "/journey"),
			Icon: lucide.Route(html.Class("size-4")),
		})
	}
	// Training Schedule (6.2.1.2) → /trainings. DIKEMBALIKAN ke menu (BL-168,
	// 17 Sep) — sempat dipause bareng Implementation Tracker (BL-78), tapi
	// user minta HANYA ini yang tampil lagi. Gate = canTrainings (BL-169:
	// objek Casbin SENDIRI `crm:adoption`, TERPISAH dari canJourney — dulu
	// numpang crm:journey bareng Customer Journey); tanpa izin → tak ditampilkan.
	if canTrainings {
		children = append(children, ui.NavItem{
			Label: "Training Schedule", Href: wsPath(slug, "/trainings"),
			Icon: lucide.CalendarCheck(html.Class("size-4")),
		})
	}
	// Success Plans → /success-plans (canViewSuccessPlans, objek crm:success_plans read).
	// Berbackend sejak slice 6.3; tanpa izin → tak ditampilkan.
	if canSuccessPlans {
		children = append(children, ui.NavItem{
			Label: "Success Plans", Href: wsPath(slug, "/success-plans"),
			Icon: lucide.ClipboardList(html.Class("size-4")),
		})
	}
	// Engagements → /engagements (canViewEngagements, objek crm:engagements read).
	// Berbackend sejak slice 6.5; tanpa izin → tak ditampilkan.
	if canEngagements {
		children = append(children, ui.NavItem{
			Label: "Engagements", Href: wsPath(slug, "/engagements"),
			Icon: lucide.MessageSquare(html.Class("size-4")),
		})
	}
	// Renewal Management → /renewal-management (canViewCSRenewals, crm:renewal_mgmt read).
	// Berbackend sejak slice 6.6; tanpa izin → tak ditampilkan.
	if canRenewals {
		children = append(children, ui.NavItem{
			Label: "Renewal Management", Href: wsPath(slug, "/renewal-management"),
			Icon: lucide.CalendarClock(html.Class("size-4")),
		})
	}
	// Playbooks → /playbooks (canViewPlaybooks, objek crm:playbooks).
	// Berbackend sejak slice A2; tanpa izin → tak ditampilkan.
	if canPlaybooks {
		children = append(children, ui.NavItem{
			Label: "Playbooks", Href: wsPath(slug, "/playbooks"),
			Icon: lucide.BookOpen(html.Class("size-4")),
		})
	}
	// Tickets / Cases → /tickets (canViewTickets, objek crm:tickets read).
	// Berbackend sejak slice B2; tanpa izin → tak ditampilkan.
	if canTickets {
		children = append(children, ui.NavItem{
			Label: "Tickets / Cases", Href: wsPath(slug, "/tickets"),
			Icon: lucide.Ticket(html.Class("size-4")),
		})
	}
	// Knowledge Base → /kb-articles (canViewKBArticles, objek crm:kb).
	// Berbackend sejak slice A3; tanpa izin → tak ditampilkan. Posisi TETAP di
	// urutan wireframe (6.10, antara Tickets/Cases & SLA Management).
	if canKB {
		children = append(children, ui.NavItem{
			Label: "Knowledge Base", Href: wsPath(slug, "/kb-articles"),
			Icon: lucide.BookMarked(html.Class("size-4")),
		})
	}
	// SLA Management → /sla-policies (canViewSLAPolicies, objek crm:sla).
	// Berbackend sejak slice A1; tanpa izin → tak ditampilkan.
	if canSLA {
		children = append(children, ui.NavItem{
			Label: "SLA Management", Href: wsPath(slug, "/sla-policies"),
			Icon: lucide.Timer(html.Class("size-4")),
		})
	}
	return &ui.NavItem{
		Label: "Customer Success", Icon: lucide.Heart(html.Class("size-4")), Children: children,
	}
}
