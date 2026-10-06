package gantt

import (
	"strconv"
	"strings"
	"time"
)

// dayjsTokens maps the dayjs format tokens Mermaid's dateFormat uses to Go
// layout elements, longest first so "MMMM" wins over "MM".
var dayjsTokens = []struct{ tok, layout string }{
	{"YYYY", "2006"}, {"MMMM", "January"}, {"dddd", "Monday"}, {"SSS", "000"},
	{"MMM", "Jan"}, {"ddd", "Mon"}, {"YY", "06"}, {"MM", "01"}, {"DD", "02"},
	{"HH", "15"}, {"hh", "03"}, {"mm", "04"}, {"ss", "05"}, {"ZZ", "-0700"},
	{"M", "1"}, {"D", "2"}, {"H", "15"}, {"h", "3"}, {"m", "4"}, {"s", "5"},
	{"A", "PM"}, {"a", "pm"}, {"Z", "-07:00"},
}

// goLayout converts a dayjs format to a Go time layout. Text in [brackets]
// is literal, as in dayjs.
func goLayout(format string) string {
	var b strings.Builder
	for i := 0; i < len(format); {
		if format[i] == '[' {
			if j := strings.IndexByte(format[i:], ']'); j > 0 {
				b.WriteString(format[i+1 : i+j])
				i += j + 1
				continue
			}
		}
		matched := false
		for _, t := range dayjsTokens {
			if strings.HasPrefix(format[i:], t.tok) {
				b.WriteString(t.layout)
				i += len(t.tok)
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

// isoLayouts are tried when a date does not match dateFormat, as Mermaid
// falls back to the JavaScript date parser.
var isoLayouts = []string{"2006-01-02", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339}

// parseDate reads s in the dayjs format, falling back to ISO forms. Times
// are UTC so the output does not depend on the host's zone.
func parseDate(s, format string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	switch strings.TrimSpace(format) {
	case "X", "x":
		n, err := strconv.ParseFloat(s, 64)
		if err != nil || n < -1e13 || n > 1e13 {
			break
		}
		if strings.TrimSpace(format) == "x" {
			n /= 1000
		}
		return time.Unix(int64(n), 0).UTC(), true
	}
	if v, err := time.ParseInLocation(goLayout(strings.TrimSpace(format)), s, time.UTC); err == nil {
		return v, true
	}
	for _, l := range isoLayouts {
		if v, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return v.UTC(), true
		}
	}
	return time.Time{}, false
}

// strftime formats t with a d3 time format, the language of axisFormat.
func strftime(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		pad := byte(0) // 0: the directive's own padding
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			pad = format[i]
			if i+1 >= len(format) {
				break
			}
			i++
		}
		num := func(n, width int, def byte) {
			s := strconv.Itoa(n)
			p := def
			if pad != 0 {
				p = pad
			}
			if p == '-' {
				b.WriteString(s)
				return
			}
			fill := "0"
			if p == '_' {
				fill = " "
			}
			for len(s) < width {
				s = fill + s
			}
			b.WriteString(s)
		}
		switch format[i] {
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'b', 'h':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'd':
			num(t.Day(), 2, '0')
		case 'e':
			num(t.Day(), 2, '_')
		case 'H':
			num(t.Hour(), 2, '0')
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			num(h, 2, '0')
		case 'j':
			num(t.YearDay(), 3, '0')
		case 'm':
			num(int(t.Month()), 2, '0')
		case 'M':
			num(t.Minute(), 2, '0')
		case 'S':
			num(t.Second(), 2, '0')
		case 'L':
			num(t.Nanosecond()/1e6, 3, '0')
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'y':
			num(t.Year()%100, 2, '0')
		case 'Y':
			num(t.Year(), 4, '0')
		case 'w':
			num(int(t.Weekday()), 1, '0')
		case 'u':
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			num(wd, 1, '0')
		case 'U':
			num((t.YearDay()+6-int(t.Weekday()))/7, 2, '0')
		case 'W':
			num((t.YearDay()+6-(int(t.Weekday())+6)%7)/7, 2, '0')
		case 'V':
			_, w := t.ISOWeek()
			num(w, 2, '0')
		case 'q':
			num((int(t.Month())+2)/3, 1, '0')
		case 'Z':
			b.WriteString("+0000")
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}
