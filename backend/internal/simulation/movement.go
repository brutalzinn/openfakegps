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
		sim.currentSpeed = 0
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

	dtSec := dt.Seconds()

	// Compute the desired target speed considering turns, stops, and end-of-route.
	targetSpeed := sim.Config.SpeedMps
	targetSpeed = adjustSpeedForTurn(sim, targetSpeed)
	targetSpeed = adjustSpeedNearEnd(sim, targetSpeed)
	targetSpeed = adjustSpeedForStops(sim, targetSpeed)

	// Ramp currentSpeed toward targetSpeed using acceleration/deceleration limits.
	sim.currentSpeed = rampSpeed(sim.currentSpeed, targetSpeed, sim.Config.AccelMps2, sim.Config.DecelMps2, dtSec)

	// Distance to travel this tick.
	dist := sim.currentSpeed * dtSec
	sim.distanceTraveled += dist

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
		sim.currentSpeed = 0
	} else {
		from := route[sim.waypointIdx]
		to := route[sim.waypointIdx+1]
		lat, lon = geo.Interpolate(from.Lat, from.Lon, to.Lat, to.Lon, sim.segmentFrac)
	}

	// Bearing: compute target bearing, then smooth toward it.
	bearing := sim.prevBearing
	if sim.waypointIdx < len(route)-1 {
		to := route[sim.waypointIdx+1]
		targetBearing := geo.Bearing(lat, lon, to.Lat, to.Lon)
		bearing = smoothBearing(sim.prevBearing, targetBearing, sim.Config.BearingSmooth)
	}
	sim.prevBearing = bearing

	return Position{
		Lat:       lat,
		Lon:       lon,
		Speed:     sim.currentSpeed,
		Bearing:   bearing,
		Altitude:  0,
		Timestamp: time.Now(),
	}
}

// rampSpeed moves current speed toward target using acceleration and deceleration limits.
func rampSpeed(current, target, accel, decel, dtSec float64) float64 {
	if current < target {
		current += accel * dtSec
		if current > target {
			current = target
		}
	} else if current > target {
		current -= decel * dtSec
		if current < target {
			current = target
		}
	}
	if current < 0 {
		current = 0
	}
	return current
}

// smoothBearing interpolates between previous and target bearing.
// factor is 0-1: 0 means snap instantly to target, 1 means never change.
func smoothBearing(prev, target, factor float64) float64 {
	// Compute shortest angular difference.
	diff := target - prev
	if diff > 180 {
		diff -= 360
	} else if diff < -180 {
		diff += 360
	}

	result := prev + diff*(1-factor)
	return math.Mod(result+360, 360)
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

// adjustSpeedNearEnd decelerates as the simulation approaches the end of the route.
func adjustSpeedNearEnd(sim *Simulation, speed float64) float64 {
	route := sim.Config.Route
	if sim.waypointIdx >= len(route)-1 {
		return 0
	}

	// Only apply on the final segment.
	if sim.waypointIdx != len(route)-2 {
		return speed
	}

	from := route[sim.waypointIdx]
	to := route[sim.waypointIdx+1]

	segLen := geo.HaversineDistance(from.Lat, from.Lon, to.Lat, to.Lon)
	remaining := segLen * (1 - sim.segmentFrac)

	// Decelerate in the last 20 meters of the final segment.
	if remaining < 20 {
		factor := remaining / 20.0
		if factor < 0.1 {
			factor = 0.1
		}
		speed *= factor
	}

	return speed
}

// adjustSpeedForStops reduces target speed when approaching a planned stop waypoint.
func adjustSpeedForStops(sim *Simulation, speed float64) float64 {
	if len(sim.Config.Stops) == 0 {
		return speed
	}

	route := sim.Config.Route
	for _, stop := range sim.Config.Stops {
		if stop.WaypointIndex <= sim.waypointIdx || stop.WaypointIndex >= len(route) {
			continue
		}

		// Compute distance from current position to the stop waypoint.
		distToStop := distanceToWaypoint(sim, stop.WaypointIndex)

		// Braking distance: v² / (2*decel)
		brakingDist := (speed * speed) / (2 * sim.Config.DecelMps2)
		if brakingDist < 20 {
			brakingDist = 20
		}

		if distToStop < brakingDist {
			factor := distToStop / brakingDist
			if factor < 0.1 {
				factor = 0.1
			}
			speed *= factor
		}
	}

	return speed
}

// distanceToWaypoint computes the remaining distance in meters from the current
// position to a specific waypoint index.
func distanceToWaypoint(sim *Simulation, targetIdx int) float64 {
	route := sim.Config.Route
	if sim.waypointIdx >= targetIdx {
		return 0
	}

	// Remaining distance in current segment.
	from := route[sim.waypointIdx]
	to := route[sim.waypointIdx+1]
	segLen := geo.HaversineDistance(from.Lat, from.Lon, to.Lat, to.Lon)
	dist := segLen * (1 - sim.segmentFrac)

	// Add full segment lengths between current+1 and targetIdx.
	for i := sim.waypointIdx + 1; i < targetIdx && i < len(route)-1; i++ {
		dist += geo.HaversineDistance(route[i].Lat, route[i].Lon, route[i+1].Lat, route[i+1].Lon)
	}

	return dist
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
