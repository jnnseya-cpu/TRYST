package profilesvc

import (
	"sort"

	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// Area is a place a member can choose as their location. Members pick an area from this
// list instead of sharing GPS: the profile then holds only that area's geohash-5 cell.
type Area struct {
	ID       string `json:"id"`
	District string `json:"district"`
	City     string `json:"city"`
	Cell     string `json:"-"`
}

func (a Area) Label(cityOnly bool) string {
	if cityOnly {
		return a.City
	}
	return a.District + ", " + a.City
}

type place struct {
	district, city string
	lat, lon       float64
}

// Launch-city districts (approximate centres; the profile stores only the 5 km cell).
var places = []place{
	{"Camden", "London", 51.5390, -0.1426}, {"Islington", "London", 51.5362, -0.1033},
	{"Hackney", "London", 51.5450, -0.0553}, {"Shoreditch", "London", 51.5265, -0.0798},
	{"Westminster", "London", 51.4975, -0.1357}, {"Kensington", "London", 51.5009, -0.1925},
	{"Fulham", "London", 51.4734, -0.2017}, {"Clapham", "London", 51.4618, -0.1384},
	{"Brixton", "London", 51.4613, -0.1156}, {"Greenwich", "London", 51.4826, -0.0077},
	{"Canary Wharf", "London", 51.5054, -0.0235}, {"Stratford", "London", 51.5413, -0.0032},
	{"Richmond", "London", 51.4613, -0.3037}, {"Wimbledon", "London", 51.4214, -0.2064},
	{"Croydon", "London", 51.3762, -0.0982}, {"Ealing", "London", 51.5130, -0.3089},
	{"City Centre", "Manchester", 53.4808, -2.2426}, {"Didsbury", "Manchester", 53.4166, -2.2310},
	{"Salford Quays", "Manchester", 53.4710, -2.2950}, {"Chorlton", "Manchester", 53.4420, -2.2780},
	{"City Centre", "Birmingham", 52.4862, -1.8904}, {"Edgbaston", "Birmingham", 52.4520, -1.9300},
	{"Moseley", "Birmingham", 52.4460, -1.8870}, {"Sutton Coldfield", "Birmingham", 52.5630, -1.8220},
	{"Central", "Brighton", 50.8225, -0.1372}, {"Hove", "Brighton", 50.8279, -0.1688},
	{"Kemptown", "Brighton", 50.8180, -0.1180},
}

// Areas returns the selectable areas, sorted by city then district.
func Areas() []Area {
	out := make([]Area, 0, len(places))
	for _, p := range places {
		cell := profile.Encode(p.lat, p.lon, 5)
		out = append(out, Area{ID: cell + ":" + p.district, District: p.district, City: p.city, Cell: cell})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].City != out[j].City {
			return out[i].City < out[j].City
		}
		return out[i].District < out[j].District
	})
	return out
}

func areaByID(id string) (Area, bool) {
	for _, a := range Areas() {
		if a.ID == id {
			return a, true
		}
	}
	return Area{}, false
}
