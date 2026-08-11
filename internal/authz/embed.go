package authz

import _ "embed"

// Model dan Policy di-embed dari file — di-load saat startup di main.go via
// authz.New(authz.Model, authz.Policy). Versionable di git.
var (
	//go:embed model.conf
	Model string

	//go:embed policy.csv
	Policy string

	// Sumbu BISNIS (CRM) — enforcer TERPISAH dari tenant/platform. Model &
	// policynya sengaja tak menumpang Model/Policy di atas: sumbu tenant punya
	// god-mode root + warisan staff→owner yang, bila diwarisi izin bisnis,
	// membuat owner/super_admin jadi "pemilik bayangan seluruh desa" (§3).
	//go:embed business.conf
	BusinessModel string

	//go:embed business_policy.csv
	BusinessPolicy string
)
