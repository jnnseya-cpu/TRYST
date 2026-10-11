package profile

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Location shown on a profile (FR-065, FR-092; 02 §1).
//
// Other members see an area name ("Islington, London", or just "London" if the member
// prefers) and a distance band ("5–10 km away"). The server only ever holds a geohash-5
// cell (about 5 km × 5 km); exact coordinates never leave the phone except in the Panic
// flow. Because every input is a whole cell, no sequence of lookups can locate a member
// more precisely than their cell (the trilateration test in 03 §9).

const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

var ErrBadCell = errors.New("invalid geohash cell")

// Encode returns the geohash of a point at the given precision. Used on the device to
// turn a position into a cell before anything is sent.
func Encode(lat, lon float64, precision int) string {
	latR, lonR := [2]float64{-90, 90}, [2]float64{-180, 180}
	var sb strings.Builder
	bit, ch, even := 0, 0, true
	for sb.Len() < precision {
		r, v := &latR, lat
		if even {
			r, v = &lonR, lon
		}
		mid := (r[0] + r[1]) / 2
		if v >= mid {
			ch |= 1 << (4 - bit)
			r[0] = mid
		} else {
			r[1] = mid
		}
		even = !even
		if bit++; bit == 5 {
			sb.WriteByte(base32[ch])
			bit, ch = 0, 0
		}
	}
	return sb.String()
}

// centre decodes a geohash cell to its centre point.
func centre(cell string) (float64, float64, error) {
	if cell == "" {
		return 0, 0, ErrBadCell
	}
	latR, lonR := [2]float64{-90, 90}, [2]float64{-180, 180}
	even := true
	for _, c := range cell {
		idx := strings.IndexRune(base32, c)
		if idx < 0 {
			return 0, 0, ErrBadCell
		}
		for b := 4; b >= 0; b-- {
			r := &latR
			if even {
				r = &lonR
			}
			mid := (r[0] + r[1]) / 2
			if idx&(1<<b) != 0 {
				r[0] = mid
			} else {
				r[1] = mid
			}
			even = !even
		}
	}
	return (latR[0] + latR[1]) / 2, (lonR[0] + lonR[1]) / 2, nil
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

// DistanceBand is the only distance another member is shown. Cells are truncated to
// geohash-5 first, so finer cells cannot leak more.
func DistanceBand(viewerCell, memberCell string) (string, error) {
	if len(viewerCell) > 5 {
		viewerCell = viewerCell[:5]
	}
	if len(memberCell) > 5 {
		memberCell = memberCell[:5]
	}
	la1, lo1, err := centre(viewerCell)
	if err != nil {
		return "", err
	}
	la2, lo2, err := centre(memberCell)
	if err != nil {
		return "", err
	}
	km := haversineKm(la1, lo1, la2, lo2)
	// Cell centres can be up to ~7 km from the true positions, so the label is always
	// approximate: "Nearby" for the same cell, otherwise rounded to 5 km (10 km beyond 50).
	switch {
	case viewerCell == memberCell:
		return "Nearby", nil
	case km < 50:
		return fmt.Sprintf("About %d km away", max(5, int(math.Round(km/5))*5)), nil
	}
	return fmt.Sprintf("About %d km away", int(math.Round(km/10))*10), nil
}

// AreaLabel resolves a cell to the area name shown on the profile. names maps a cell
// prefix (geohash-4 or -5) to "District, City"; cityOnly drops the district for members who
// prefer less detail.
func AreaLabel(cell string, names map[string]string, cityOnly bool) string {
	if len(cell) > 5 {
		cell = cell[:5]
	}
	for n := len(cell); n >= 3; n-- {
		if name, ok := names[cell[:n]]; ok {
			if cityOnly {
				if i := strings.LastIndex(name, ", "); i >= 0 {
					return name[i+2:]
				}
			}
			return name
		}
	}
	return "Location hidden"
}
