package gantt

import (
	"regexp"
	"strconv"
	"time"
)

// interval is an axis tick step: n of a calendar or clock unit.
type interval struct {
	unit byte // 's' second, 'm' minute, 'h' hour, 'd' day, 'w' week, 'M' month, 'y' year
	n    int
}

func (iv interval) approx() time.Duration {
	day := 24 * time.Hour
	u := map[byte]time.Duration{'S': time.Millisecond, 's': time.Second, 'm': time.Minute, 'h': time.Hour, 'd': day, 'w': 7 * day, 'M': 30 * day, 'y': 365 * day}[iv.unit]
	return u * time.Duration(iv.n)
}

// autoIntervals are the steps d3 picks from for a time axis.
var autoIntervals = []interval{
	{'s', 1}, {'s', 5}, {'s', 15}, {'s', 30}, {'m', 1}, {'m', 5}, {'m', 15}, {'m', 30},
	{'h', 1}, {'h', 3}, {'h', 6}, {'h', 12}, {'d', 1}, {'d', 2}, {'w', 1}, {'M', 1}, {'M', 3},
	{'y', 1}, {'y', 2}, {'y', 5}, {'y', 10}, {'y', 20}, {'y', 50}, {'y', 100},
}

var tickIntervalRe = regexp.MustCompile(`^([1-9][0-9]{0,3})(millisecond|second|minute|hour|day|week|month)$`)

// parseTickInterval reads Mermaid's tickInterval value, e.g. "1week".
func parseTickInterval(s string) (interval, bool) {
	m := tickIntervalRe.FindStringSubmatch(s)
	if m == nil {
		return interval{}, false
	}
	n, _ := strconv.Atoi(m[1])
	u := map[string]byte{"millisecond": 'S', "second": 's', "minute": 'm', "hour": 'h', "day": 'd', "week": 'w', "month": 'M'}[m[2]]
	if u == 'S' {
		// Milliseconds are below anything a gantt bar can show; use seconds.
		u, n = 's', max(1, n/1000)
	}
	return interval{u, n}, true
}

// floor returns the latest tick boundary at or before t.
func (iv interval) floor(t time.Time, weekStart time.Weekday) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	switch iv.unit {
	case 's':
		return t.Truncate(time.Duration(iv.n) * time.Second)
	case 'm':
		return t.Truncate(time.Duration(iv.n) * time.Minute)
	case 'h':
		return day.Add(t.Sub(day).Truncate(time.Duration(iv.n) * time.Hour))
	case 'd':
		for (day.Day()-1)%iv.n != 0 {
			day = day.AddDate(0, 0, -1)
		}
		return day
	case 'w':
		back := (int(day.Weekday()) - int(weekStart) + 7) % 7
		return day.AddDate(0, 0, -back)
	case 'M':
		m := (int(t.Month()) - 1) / iv.n * iv.n
		return time.Date(t.Year(), time.Month(m+1), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(t.Year()/iv.n*iv.n, 1, 1, 0, 0, 0, 0, time.UTC)
	}
}

// next returns the tick boundary after the boundary t.
func (iv interval) next(t time.Time) time.Time {
	switch iv.unit {
	case 's':
		return t.Add(time.Duration(iv.n) * time.Second)
	case 'm':
		return t.Add(time.Duration(iv.n) * time.Minute)
	case 'h':
		n := t.Add(time.Duration(iv.n) * time.Hour)
		if n.Day() != t.Day() {
			return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
		}
		return n
	case 'd':
		// d3 steps days by day of month, starting again on the 1st.
		n := t.AddDate(0, 0, 1)
		for (n.Day()-1)%iv.n != 0 {
			n = n.AddDate(0, 0, 1)
		}
		return n
	case 'w':
		return t.AddDate(0, 0, 7*iv.n)
	case 'M':
		return t.AddDate(0, iv.n, 0)
	default:
		return t.AddDate(iv.n, 0, 0)
	}
}

// maxTicks bounds every tick walk.
const maxTicks = 400

// ticks lists the boundaries of iv inside [lo, hi].
func (iv interval) ticks(lo, hi time.Time, weekStart time.Weekday) []time.Time {
	var out []time.Time
	t := iv.floor(lo, weekStart)
	for i := 0; i < maxTicks*4 && !t.After(hi); i++ {
		if !t.Before(lo) {
			out = append(out, t)
			if len(out) >= maxTicks {
				break
			}
		}
		t = iv.next(t)
	}
	return out
}
