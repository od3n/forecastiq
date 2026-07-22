package domain

import "github.com/google/uuid"

// Well-known seeded reference identities (migration/seed + services share
// these). They are fixed UUIDs (not generated) so a fresh database and the
// application agree on the system workspace, seeded providers, the Open-Meteo
// configuration, and the demo location (Johor Bahru).
var (
	SystemWorkspaceID     = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	OpenMeteoProviderID   = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	OpenWeatherProviderID = uuid.MustParse("00000000-0000-0000-0000-000000000011")
	OpenMeteoConfigID     = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	JohorBahruLocationID  = uuid.MustParse("00000000-0000-0000-0000-000000000030")
)
