package geo

import "math"

const (
	// EarthRadiusMeters is the mean radius of Earth in meters.
	EarthRadiusMeters = 6_371_000.0
)

// HaversineDistance returns the distance in meters between two lat/lon points.
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)

	rLat1 := toRad(lat1)
	rLat2 := toRad(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusMeters * c
}

// Bearing returns the initial bearing in degrees from point 1 to point 2.
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	rLat1 := toRad(lat1)
	rLat2 := toRad(lat2)
	dLon := toRad(lon2 - lon1)

	y := math.Sin(dLon) * math.Cos(rLat2)
	x := math.Cos(rLat1)*math.Sin(rLat2) - math.Sin(rLat1)*math.Cos(rLat2)*math.Cos(dLon)

	bearing := toDeg(math.Atan2(y, x))
	return math.Mod(bearing+360, 360)
}

// Interpolate returns a point along the great circle between two points at the
// given fraction (0.0 = start, 1.0 = end).
func Interpolate(lat1, lon1, lat2, lon2, fraction float64) (float64, float64) {
	if fraction <= 0 {
		return lat1, lon1
	}
	if fraction >= 1 {
		return lat2, lon2
	}

	rLat1 := toRad(lat1)
	rLon1 := toRad(lon1)
	rLat2 := toRad(lat2)
	rLon2 := toRad(lon2)

	dLat := rLat2 - rLat1
	dLon := rLon2 - rLon1

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	d := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	if d < 1e-12 {
		return lat1, lon1
	}

	aa := math.Sin((1-fraction)*d) / math.Sin(d)
	bb := math.Sin(fraction*d) / math.Sin(d)

	x := aa*math.Cos(rLat1)*math.Cos(rLon1) + bb*math.Cos(rLat2)*math.Cos(rLon2)
	y := aa*math.Cos(rLat1)*math.Sin(rLon1) + bb*math.Cos(rLat2)*math.Sin(rLon2)
	z := aa*math.Sin(rLat1) + bb*math.Sin(rLat2)

	lat := math.Atan2(z, math.Sqrt(x*x+y*y))
	lon := math.Atan2(y, x)

	return toDeg(lat), toDeg(lon)
}

// DecodePolyline decodes a Google encoded polyline string into a slice of
// lat/lon pairs. See: https://developers.google.com/maps/documentation/utilities/polylinealgorithm
func DecodePolyline(encoded string) [][2]float64 {
	var points [][2]float64
	index := 0
	lat, lng := 0, 0

	for index < len(encoded) {
		// Decode latitude
		shift, result := 0, 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lat += ^(result >> 1)
		} else {
			lat += result >> 1
		}

		// Decode longitude
		shift, result = 0, 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lng += ^(result >> 1)
		} else {
			lng += result >> 1
		}

		points = append(points, [2]float64{float64(lat) / 1e5, float64(lng) / 1e5})
	}

	return points
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}

func toDeg(rad float64) float64 {
	return rad * 180 / math.Pi
}
