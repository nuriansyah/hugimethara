package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata" // schedule triggers resolve IANA zones even in minimal images
)

// Cron is a parsed five-field cron expression: minute hour day-of-month month
// day-of-week. Fields accept *, numbers, ranges (a-b), lists (a,b) and steps
// (*/n, a-b/n). Day-of-week is 0–6 (Sunday = 0; 7 is also Sunday); month and
// weekday names (jan, mon…) are accepted. As in Vixie cron, when both
// day-of-month and day-of-week are restricted a day matches either.
type Cron struct {
	expr                         string
	minute, hour, dom, month, dw uint64
	domStar, dowStar             bool
}

// String returns the source expression.
func (c Cron) String() string { return c.expr }

var cronNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10,
	"nov": 11, "dec": 12, "sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// ParseCron parses a five-field expression.
func ParseCron(expr string) (Cron, error) {
	fields := strings.Fields(strings.ToLower(expr))
	if len(fields) != 5 {
		return Cron{}, fmt.Errorf("cron must have 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	c := Cron{expr: strings.Join(fields, " "), domStar: fields[2] == "*", dowStar: fields[4] == "*"}
	specs := []struct {
		dst      *uint64
		min, max int
		name     string
	}{
		{&c.minute, 0, 59, "minute"}, {&c.hour, 0, 23, "hour"}, {&c.dom, 1, 31, "day-of-month"},
		{&c.month, 1, 12, "month"}, {&c.dw, 0, 7, "day-of-week"},
	}
	for i, s := range specs {
		bits, err := cronField(fields[i], s.min, s.max)
		if err != nil {
			return Cron{}, fmt.Errorf("cron %s: %w", s.name, err)
		}
		*s.dst = bits
	}
	if c.dw&(1<<7) != 0 {
		c.dw |= 1 // 7 is Sunday
	}
	return c, nil
}

func cronValue(s string, min, max int) (int, error) {
	if v, ok := cronNames[s]; ok {
		return v, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("%q is not a value between %d and %d", s, min, max)
	}
	return v, nil
}

func cronField(field string, min, max int) (uint64, error) {
	var bits uint64
	for _, part := range strings.Split(field, ",") {
		rng, step := part, 1
		if i := strings.IndexByte(part, '/'); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n < 1 || n > max {
				return 0, fmt.Errorf("invalid step in %q", part)
			}
			rng, step = part[:i], n
		}
		lo, hi := min, max
		switch {
		case rng == "*":
		case strings.Contains(rng, "-"):
			a, b, _ := strings.Cut(rng, "-")
			var err error
			if lo, err = cronValue(a, min, max); err != nil {
				return 0, err
			}
			if hi, err = cronValue(b, min, max); err != nil {
				return 0, err
			}
			if lo > hi {
				return 0, fmt.Errorf("range %q is reversed", rng)
			}
		default:
			v, err := cronValue(rng, min, max)
			if err != nil {
				return 0, err
			}
			lo = v
			if step == 1 {
				hi = v
			}
		}
		for v := lo; v <= hi; v += step {
			bits |= 1 << uint(v)
		}
	}
	return bits, nil
}

func (c Cron) dayMatches(t time.Time) bool {
	dom := c.dom&(1<<uint(t.Day())) != 0
	dow := c.dw&(1<<uint(t.Weekday())) != 0
	switch {
	case c.domStar && c.dowStar:
		return true
	case c.domStar:
		return dow
	case c.dowStar:
		return dom
	}
	return dom || dow
}

// Next returns the first activation strictly after `after`, evaluated in loc.
// It returns the zero time if none exists within five years (e.g. 30 February).
func (c Cron) Next(after time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	t := after.In(loc).Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if c.month&(1<<uint(t.Month())) == 0 {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !c.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if c.hour&(1<<uint(t.Hour())) == 0 {
			// Add, not Date: stepping by wall-clock hours loops on DST fall-back.
			// Not Truncate either: it is absolute and breaks half-hour zones.
			t = t.Add(time.Duration(60-t.Minute()) * time.Minute)
			continue
		}
		if c.minute&(1<<uint(t.Minute())) == 0 {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}
