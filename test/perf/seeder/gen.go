// Deterministic synthetic-weather generation (WP-26b). All values derive from
// a splitmix64 hash of (seed, location, provider, hour), so the SAME flag set
// always produces the SAME physically-plausible tropical weather regardless of
// iteration order or wall clock. Only the time anchor (rows end at the current
// hour) depends on the run instant.
//
// Reference: docs/testing/04-performance-testing.md §3 ("deterministic (fixed
// seed) Go program generating physically plausible tropical weather values").
package main

import (
	"encoding/binary"
	"math"

	"github.com/google/uuid"
)

// idNamespace is the fixed namespace for deterministic row ids (uuid.NewSHA1).
// Deterministic ids let the match pass re-derive snapshot/observation ids
// without holding millions of rows in memory.
var idNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// splitmix64 is the standard 64-bit mix used as the deterministic value core.
func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e91d
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// hash combines the run seed with arbitrary discriminators.
func hash(seed int64, parts ...uint64) uint64 {
	h := splitmix64(uint64(seed))
	for _, p := range parts {
		h = splitmix64(h ^ p)
	}
	return h
}

// unit maps a hash to [0, 1).
func unit(h uint64) float64 { return float64(h>>11) / float64(1<<53) }

// weather holds the "true" state of the atmosphere at one location-hour; the
// observation records it and forecasts approximate it with horizon-growing
// error.
type weather struct {
	TempC       float64
	HumidityPct float64
	WindMS      float64
	PressureHPA float64
	PrecipMM    float64
}

// trueWeather generates the ground-truth tropical weather for a location-hour.
// Diurnal temperature sinusoid (24–33 °C), humid (60–98 %), light wind, narrow
// equatorial pressure band, and bursty afternoon convective rain.
func trueWeather(seed int64, locIdx int, unixHour int64) weather {
	h := hash(seed, 0x57454154, uint64(locIdx), uint64(unixHour))
	hourOfDay := float64(((unixHour % 24) + 24) % 24)
	// Peak heat ~14:00 local (locations are seeded around UTC+8).
	diurnal := math.Sin((hourOfDay - 6 + 8) / 24 * 2 * math.Pi)

	temp := 28.5 + 4.0*diurnal + 1.5*(unit(h)-0.5)
	humidity := 82 - 14*diurnal + 8*(unit(splitmix64(h+1))-0.5)
	wind := 2.5 + 2.0*unit(splitmix64(h+2))
	pressure := 1009 + 3.0*(unit(splitmix64(h+3))-0.5)

	// Afternoon convection: ~30 % of afternoon hours rain, heavier bursts.
	precip := 0.0
	rainRoll := unit(splitmix64(h + 4))
	if hourOfDay >= 13 && hourOfDay <= 19 {
		if rainRoll < 0.30 {
			precip = 0.5 + 12*unit(splitmix64(h+5))
		}
	} else if rainRoll < 0.08 {
		precip = 0.2 + 4*unit(splitmix64(h+5))
	}
	return weather{
		TempC:       round2(temp),
		HumidityPct: round2(clamp(humidity, 0, 100)),
		WindMS:      round2(wind),
		PressureHPA: round2(pressure),
		PrecipMM:    round2(precip),
	}
}

// forecastFor derives a provider forecast for a target hour: the truth plus a
// horizon-growing deterministic error (providers differ via provIdx salt).
type forecast struct {
	weather
	PrecipProb float64
}

func forecastFor(seed int64, provIdx, locIdx int, issuedUnixHour, targetUnixHour int64) forecast {
	truth := trueWeather(seed, locIdx, targetUnixHour)
	horizonH := float64(targetUnixHour - issuedUnixHour)
	h := hash(seed, 0x464f5245, uint64(provIdx), uint64(locIdx), uint64(issuedUnixHour), uint64(targetUnixHour))

	// Error stddev grows with horizon; provider 1 is slightly worse than 0.
	scale := (0.4 + 0.02*horizonH) * (1 + 0.25*float64(provIdx))
	e := func(salt uint64) float64 { return (unit(splitmix64(h+salt)) - 0.5) * 2 * scale }

	f := forecast{weather: weather{
		TempC:       round2(truth.TempC + e(1)),
		HumidityPct: round2(clamp(truth.HumidityPct+4*e(2), 0, 100)),
		WindMS:      round2(math.Max(0, truth.WindMS+0.5*e(3))),
		PressureHPA: round2(truth.PressureHPA + e(4)),
		PrecipMM:    round2(math.Max(0, truth.PrecipMM+2*e(5))),
	}}
	// Probability correlated with the forecast amount, in [0, 1].
	if f.PrecipMM > 0.1 {
		f.PrecipProb = round4(clamp(0.45+f.PrecipMM/20+0.2*e(6), 0, 1))
	} else {
		f.PrecipProb = round4(clamp(0.08+0.1*unit(splitmix64(h+7)), 0, 1))
	}
	return f
}

// correctionDelta is the small revision a corrected observation applies.
func correctionDelta(seed int64, locIdx int, unixHour int64) float64 {
	h := hash(seed, 0x434f5252, uint64(locIdx), uint64(unixHour))
	return round2((unit(h) - 0.5) * 0.4)
}

// suspectObservation flags ~2 % of hours as suspect (OC-04 realism).
func suspectObservation(seed int64, locIdx int, unixHour int64) bool {
	return unit(hash(seed, 0x53555350, uint64(locIdx), uint64(unixHour))) < 0.02
}

// Deterministic row ids (uuid v5 over a stable key) — re-derivable in the
// match pass without buffering the referenced rows.
func snapshotID(provIdx, locIdx int, issuedUnixHour, targetUnixHour int64) uuid.UUID {
	return keyedID("snap", uint64(provIdx), uint64(locIdx), uint64(issuedUnixHour), uint64(targetUnixHour))
}

func observationID(locIdx int, unixHour int64, corrected bool) uuid.UUID {
	c := uint64(0)
	if corrected {
		c = 1
	}
	return keyedID("obs", uint64(locIdx), uint64(unixHour), c)
}

func collectionID(provIdx, locIdx int, issuedUnixHour int64) uuid.UUID {
	return keyedID("coll", uint64(provIdx), uint64(locIdx), uint64(issuedUnixHour))
}

func keyedID(kind string, parts ...uint64) uuid.UUID {
	buf := make([]byte, 0, len(kind)+8*len(parts))
	buf = append(buf, kind...)
	for _, p := range parts {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], p)
		buf = append(buf, b[:]...)
	}
	return uuid.NewSHA1(idNamespace, buf)
}

func clamp(v, lo, hi float64) float64 { return math.Min(hi, math.Max(lo, v)) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
