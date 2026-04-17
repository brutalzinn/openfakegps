package simulation

import (
	"math"
	"math/rand"
)

// addNoise applies random GPS noise to a position within the given radius in meters.
func addNoise(pos Position, radiusMeters float64) Position {
	if radiusMeters <= 0 {
		return pos
	}

	// Random distance within radius (uniform distribution in circle).
	r := radiusMeters * math.Sqrt(rand.Float64())
	theta := rand.Float64() * 2 * math.Pi

	// Convert offset to lat/lon degrees.
	// 1 degree latitude ~ 111,320 meters
	// 1 degree longitude ~ 111,320 * cos(lat) meters
	dLat := (r * math.Cos(theta)) / 111320.0
	dLon := (r * math.Sin(theta)) / (111320.0 * math.Cos(pos.Lat*math.Pi/180.0))

	pos.Lat += dLat
	pos.Lon += dLon

	// Vary accuracy slightly: between 1.0 and radiusMeters*2.
	pos.Accuracy = 1.0 + rand.Float64()*(radiusMeters*2-1.0)

	// Speed noise: add gaussian-like jitter (±0.5 m/s) when moving.
	if pos.Speed > 0 {
		speedNoise := (rand.Float64()*2 - 1) * 0.5 // uniform ±0.5 m/s
		pos.Speed = math.Max(0, pos.Speed+speedNoise)
	}

	return pos
}
