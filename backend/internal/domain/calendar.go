package domain

import "time"

func BucharestLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		return time.UTC
	}
	return location
}

func IsRomanianNonWorkingDay(now time.Time, extra map[string]struct{}) bool {
	local := now.In(BucharestLocation())
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return true
	}
	if _, exists := extra[local.Format("2006-01-02")]; exists {
		return true
	}
	fixed := map[string]struct{}{
		"01-01": {}, "01-02": {}, "01-06": {}, "01-07": {}, "01-24": {},
		"05-01": {}, "06-01": {}, "08-15": {}, "11-30": {}, "12-01": {},
		"12-25": {}, "12-26": {},
	}
	if _, exists := fixed[local.Format("01-02")]; exists {
		return true
	}
	easter := orthodoxEaster(local.Year(), local.Location())
	for _, holiday := range []time.Time{easter.AddDate(0, 0, -2), easter.AddDate(0, 0, 1), easter.AddDate(0, 0, 50)} {
		if holiday.Format("2006-01-02") == local.Format("2006-01-02") {
			return true
		}
	}
	return false
}

// orthodoxEaster uses the Julian-calendar computus and converts it to the
// Gregorian calendar, which is sufficient for Romanian public holidays.
func orthodoxEaster(year int, location *time.Location) time.Time {
	a := year % 4
	b := year % 7
	c := year % 19
	d := (19*c + 15) % 30
	e := (2*a + 4*b - d + 34) % 7
	month := (d + e + 114) / 31
	day := ((d + e + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, location).AddDate(0, 0, 13)
}
