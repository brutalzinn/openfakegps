package simulation

import (
	"math"
	"time"

	"github.com/openfakegps/openfakegps/backend/pkg/geo"
)

// advancePosition moves the simulation forward along its route for the given
// time delta and returns the new position.
func advancePosition(sim *Simulation, dt time.Duration) Position {
	route := sim.Config.Route
	if len(route) < 2 {
		return sim.CurrentPos
	}

	// Check if we have already reached the end.
	if sim.waypointIdx >= len(route)-1 {
		last := route[len(route)-1]
		return Position{
			Lat:       last.Lat,
			Lon:       last.Lon,
			Speed:     0,
			Bearing:   sim.prevBearing,
			Altitude:  0,
			Timestamp: time.Now(),
		}
	}

	targetSpeed := sim.Config.SpeedMps

	// Calculate turn sharpness to modulate speed.
	targetSpeed = adjustSpeedForTurn(sim, targetSpeed)

	// Apply deceleration when approaching next waypoint.
	targetSpeed = adjustSpeedNearWaypoint(sim, targetSpeed)

	// Distance to travel this tick.
	dist := targetSpeed * dt.Seconds()

	// Walk along segments, consuming distance.
	for dist > 0 && sim.waypointIdx < len(route)-1 {
		from := route[sim.waypointIdx]
		to := route[sim.waypointIdx+1]

		segLen := geo.HaversineDistance(from.Lat, from.Lon, to.Lat, to.Lon)
		if segLen < 1e-9 {
			sim.waypointIdx++
			sim.segmentFrac = 0
			continue
		}

		remaining := segLen * (1 - sim.segmentFrac)
		if dist < remaining {
			sim.segmentFrac += dist / segLen
			dist = 0
		} else {
			dist -= remaining
			sim.waypointIdx++
			sim.segmentFrac = 0
		}
	}

	// Compute interpolated position.
	var lat, lon float64
	if sim.waypointIdx >= len(route)-1 {
		sim.waypointIdx = len(route) - 1
		last := route[sim.waypointIdx]
		lat, lon = last.Lat, last.Lon
		targetSpeed = 0
	} else {
		from := route[sim.waypointIdx]
		to := route[sim.waypointIdx+1]
		lat, lon = geo.Interpolate(from.Lat, from.Lon, to.Lat, to.Lon, sim.segmentFrac)
	}

	// Bearing from current interpolated position toward next waypoint.
	bearing := sim.prevBearing
	if sim.waypointIdx < len(route)-1 {
		to := route[sim.waypointIdx+1]
		bearing = geo.Bearing(lat, lon, to.Lat, to.Lon)
	}
	sim.prevBearing = bearing

	return Position{
		Lat:       lat,
		Lon:       lon,
		Speed:     targetSpeed,
		Bearing:   bearing,
		Altitude:  0,
		Timestamp: time.Now(),
	}
}

// adjustSpeedForTurn reduces speed when the route has a sharp turn at the
// upcoming waypoint (bearing change > 30 degrees).
func adjustSpeedForTurn(sim *Simulation, speed float64) float64 {
	route := sim.Config.Route
	idx := sim.waypointIdx

	// Need at least 3 points to compute a turn angle.
	if idx+2 >= len(route) || idx < 0 {
		return speed
	}

	b1 := geo.Bearing(route[idx].Lat, route[idx].Lon, route[idx+1].Lat, route[idx+1].Lon)
	b2 := geo.Bearing(route[idx+1].Lat, route[idx+1].Lon, route[idx+2].Lat, route[idx+2].Lon)

	angleDiff := math.Abs(b2 - b1)
	if angleDiff > 180 {
		angleDiff = 360 - angleDiff
	}

	// Reduce speed proportionally for turns sharper than 30 degrees.
	if angleDiff > 30 {
		factor := 1.0 - math.Min(angleDiff/180.0, 0.7)
		speed *= factor
	}

	return speed
}

// adjustSpeedNearWaypoint decelerates as the simulation approaches the next
// waypoint to provide smoother movement.
func adjustSpeedNearWaypoint(sim *Simulation, speed float64) float64 {
	route := sim.Config.Route
	if sim.waypointIdx >= len(route)-1 {
		return 0
	}

	from := route[sim.waypointIdx]
	to := route[sim.waypointIdx+1]

	segLen := geo.HaversineDistance(from.Lat, from.Lon, to.Lat, to.Lon)
	remaining := segLen * (1 - sim.segmentFrac)

	// Decelerate in the last 20 meters of the final segment.
	if sim.waypointIdx == len(route)-2 && remaining < 20 {
		factor := remaining / 20.0
		if factor < 0.1 {
			factor = 0.1
		}
		speed *= factor
	}

	return speed
}

// progress returns a value from 0.0 to 1.0 indicating how far along the route
// the simulation has progressed.
func progress(sim *Simulation) float64 {
	route := sim.Config.Route
	if len(route) < 2 {
		return 1.0
	}

	totalSegments := float64(len(route) - 1)
	completed := float64(sim.waypointIdx) + sim.segmentFrac

	p := completed / totalSegments
	if p > 1 {
		p = 1
	}
	return p
}
