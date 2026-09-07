package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_cs.go — grup nav Customer Success (wireframe 6, Modul 6).
// Selalu tampil (peta jalan produk terlihat, "nol menu hantu"): Health Score
// (C1), SLA Management (A1), Playbooks (A2), Knowledge Base (A3) & Tickets/Cases
// (B2) berbackend — sisanya placeholder disabled sampai slice-nya mendarat.

// workspaceCSGroup merakit grup Customer Success (wireframe 6). canSLA =
// canViewSLAPolicies (izin SAMA dengan gerbang SLAPoliciesList). canPlaybooks
// = canViewPlaybooks. canKB = canViewKBArticles. canTickets = canViewTickets
// (izin SAMA dengan gerbang TicketsList, objek crm:tickets read).
// canHealthScore = canViewHealthScore (izin SAMA dengan gerbang HealthScoreList,
// objek crm:health read). canSuccessPlans = canViewSuccessPlans (crm:success_plans read).
// canEngagements = canViewEngagements (izin SAMA dengan gerbang EngagementsList,
// objek crm:engagements read). canRenewals = canViewCSRenewals (crm:renewal_mgmt read).
// canReadCSJourney (Customer Journey / Lifecycle 6.2, REUSE crm:journey read).
//
// Implementation Tracker (6.2.1.1) & Training Schedule (6.2.1.2) SENGAJA tak
// lagi punya item nav (BL-78, level A — hanya disembunyikan dari menu). Route,
// handler, tabel & gate crm:journey (canViewImplTasks/canViewTrainings) tetap
// hidup; item mudah dikembalikan dengan menambah kembali NavItem-nya.
func workspaceCSGroup(slug string, canSLA, canPlaybooks, canKB, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals, canJourney bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 12)
	// Health Score → /health-scores (canViewHealthScore, objek crm:health read).
	// Berbackend sejak slice C1; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	healthScore := ui.NavItem{Label: "Health Score", Icon: lucide.HeartPulse(html.Class("size-4"))}
	if canHealthScore {
		healthScore.Href = wsPath(slug, "/health-scores")
	} else {
		healthScore.Disabled = true
	}
	children = append(children, healthScore)
	// Customer Journey → /journey (canReadCSJourney, crm:journey read — objek
	// SAMA dgn Implementation Tracker/Training). Dashboard portofolio fase
	// lifecycle desa (6.2, BL-77); disabled bila tak berhak, tetap tampil.
	journey := ui.NavItem{Label: "Customer Journey", Icon: lucide.Route(html.Class("size-4"))}
	if canJourney {
		journey.Href = wsPath(slug, "/journey")
	} else {
		journey.Disabled = true
	}
	children = append(children, journey)
	// Success Plans → /success-plans (canViewSuccessPlans, objek crm:success_plans read).
	// Berbackend sejak slice 6.3; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	successPlans := ui.NavItem{Label: "Success Plans", Icon: lucide.ClipboardList(html.Class("size-4"))}
	if canSuccessPlans {
		successPlans.Href = wsPath(slug, "/success-plans")
	} else {
		successPlans.Disabled = true
	}
	children = append(children, successPlans)
	// Engagements → /engagements (canViewEngagements, objek crm:engagements read).
	// Berbackend sejak slice 6.5; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	engagements := ui.NavItem{Label: "Engagements", Icon: lucide.MessageSquare(html.Class("size-4"))}
	if canEngagements {
		engagements.Href = wsPath(slug, "/engagements")
	} else {
		engagements.Disabled = true
	}
	children = append(children, engagements)
	// Renewal Management → /renewal-management (canViewCSRenewals, crm:renewal_mgmt read).
	// Berbackend sejak slice 6.6; disabled bila tak berhak, tetap tampil.
	renewal := ui.NavItem{Label: "Renewal Management", Icon: lucide.CalendarClock(html.Class("size-4"))}
	if canRenewals {
		renewal.Href = wsPath(slug, "/renewal-management")
	} else {
		renewal.Disabled = true
	}
	children = append(children, renewal)
	// Playbooks → /playbooks (canViewPlaybooks, objek crm:playbooks).
	// Berbackend sejak slice A2; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	playbooks := ui.NavItem{Label: "Playbooks", Icon: lucide.BookOpen(html.Class("size-4"))}
	if canPlaybooks {
		playbooks.Href = wsPath(slug, "/playbooks")
	} else {
		playbooks.Disabled = true
	}
	children = append(children, playbooks)
	// Tickets / Cases → /tickets (canViewTickets, objek crm:tickets read).
	// Berbackend sejak slice B2; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	tickets := ui.NavItem{Label: "Tickets / Cases", Icon: lucide.Ticket(html.Class("size-4"))}
	if canTickets {
		tickets.Href = wsPath(slug, "/tickets")
	} else {
		tickets.Disabled = true
	}
	children = append(children, tickets)
	// Knowledge Base → /kb-articles (canViewKBArticles, objek crm:kb).
	// Berbackend sejak slice A3; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat. Posisi TETAP di urutan wireframe
	// (6.10, antara Tickets/Cases & SLA Management).
	kb := ui.NavItem{Label: "Knowledge Base", Icon: lucide.BookMarked(html.Class("size-4"))}
	if canKB {
		kb.Href = wsPath(slug, "/kb-articles")
	} else {
		kb.Disabled = true
	}
	children = append(children, kb)
	// SLA Management → /sla-policies (canViewSLAPolicies, objek crm:sla).
	// Berbackend sejak slice A1; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	sla := ui.NavItem{Label: "SLA Management", Icon: lucide.Timer(html.Class("size-4"))}
	if canSLA {
		sla.Href = wsPath(slug, "/sla-policies")
	} else {
		sla.Disabled = true
	}
	children = append(children, sla)
	return ui.NavItem{
		Label: "Customer Success", Icon: lucide.Heart(html.Class("size-4")), Children: children,
	}
}
