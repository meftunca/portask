// core.go (Nihai ve Düzeltilmiş Hali)
package jcron

import (
	"fmt"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Package-Level Tanımlamalar ---
var (
	predefinedSchedules = map[string]string{
		"@yearly": "0 0 0 1 1 *", "@annually": "0 0 0 1 1 *", "@monthly": "0 0 0 1 * *",
		"@weekly": "0 0 0 * * 0", "@daily": "0 0 0 * * *", "@midnight": "0 0 0 * * *",
		"@hourly": "0 0 * * * *",
	}
	dayAbbreviations = map[string]string{
		"SUN": "0", "MON": "1", "TUE": "2", "WED": "3", "THU": "4", "FRI": "5", "SAT": "6",
	}
	monthAbbreviations = map[string]string{
		"JAN": "1", "FEB": "2", "MAR": "3", "APR": "4", "MAY": "5", "JUN": "6",
		"JUL": "7", "AUG": "8", "SEP": "9", "OCT": "10", "NOV": "11", "DEC": "12",
	}

	// Pool for strings.Builder to reduce allocations in cache key building
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// Pre-compiled common special patterns for fast lookup
	commonSpecialPatterns = map[string]bool{
		"L": true, "1L": true, "2L": true, "3L": true, "4L": true, "5L": true, "6L": true,
		"1#1": true, "1#2": true, "1#3": true, "1#4": true, "1#5": true,
		"2#1": true, "2#2": true, "2#3": true, "2#4": true, "2#5": true,
		"3#1": true, "3#2": true, "3#3": true, "3#4": true, "3#5": true,
		"4#1": true, "4#2": true, "4#3": true, "4#4": true, "4#5": true,
		"5#1": true, "5#2": true, "5#3": true, "5#4": true, "5#5": true,
		"6#1": true, "6#2": true, "6#3": true, "6#4": true, "6#5": true,
		"0#1": true, "0#2": true, "0#3": true, "0#4": true, "0#5": true,
	}
)

// --- Çekirdek Veri Yapıları ---
type Schedule struct{ Second, Minute, Hour, DayOfMonth, Month, DayOfWeek, Year, Timezone *string }
type ExpandedSchedule struct {
	SecondsMask, MinutesMask   uint64
	HoursMask, DaysOfMonthMask uint32
	MonthsMask                 uint16
	DaysOfWeekMask             uint8
	Years                      []int
	DayOfMonth, DayOfWeek      string
	Location                   *time.Location
	HasSpecialDayPatterns      bool // Cache flag for L/# patterns
}
type Engine struct {
	cache map[string]*ExpandedSchedule
	mu    sync.RWMutex // RWMutex for better read performance
}

func New() *Engine { return &Engine{cache: make(map[string]*ExpandedSchedule)} }

// --- Next() ve Prev() Uygulamaları ---
func (e *Engine) Next(schedule Schedule, fromTime time.Time) (time.Time, error) {
	exp, err := e.getExpandedSchedule(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return e.findNext(fromTime.In(exp.Location), exp)
}
func (e *Engine) Prev(schedule Schedule, fromTime time.Time) (time.Time, error) {
	exp, err := e.getExpandedSchedule(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return e.findPrev(fromTime.In(exp.Location), exp)
}

// NİHAİ VERSİYON: findNext - Sadece ileri arama yapar ve tüm durumları doğru yönetir.
func (e *Engine) findNext(fromTime time.Time, s *ExpandedSchedule) (time.Time, error) {
	t := fromTime.Add(time.Second)
	limit := fromTime.AddDate(10, 0, 0)

	for {
		if t.After(limit) {
			return time.Time{}, fmt.Errorf("no execution time found")
		}

		if !yearMatches(s, t.Year()) {
			t = advanceYear(t, s.Years, 1, s.Location)
			continue
		}
		if (s.MonthsMask & (1 << uint(t.Month()))) == 0 {
			t = advanceMonth(t, 1, s.Location)
			continue
		}
		if !e.isDayMatch(t, s) {
			t = advanceDay(t, 1, s.Location)
			continue
		}

		hour, min, sec := t.Hour(), t.Minute(), t.Second()

		// Find next valid hour
		nextHour, hourWrapped := findNextSetBit(uint64(s.HoursMask), hour, 23)
		if hourWrapped {
			t = advanceDay(t, 1, s.Location)
			continue
		}
		if nextHour > hour {
			// Jump to the next valid hour and reset minutes/seconds
			nextMin, _ := findNextSetBit(s.MinutesMask, 0, 59)
			nextSec, _ := findNextSetBit(s.SecondsMask, 0, 59)
			candidate := time.Date(t.Year(), t.Month(), t.Day(), nextHour, nextMin, nextSec, 0, s.Location)
			if candidate.After(fromTime) {
				return candidate, nil
			}
			t = candidate.Add(time.Second)
			continue
		}

		// Find next valid minute
		nextMin, minWrapped := findNextSetBit(s.MinutesMask, min, 59)
		if minWrapped {
			t = advanceHour(t, 1, s.Location)
			continue
		}
		if nextMin > min {
			// Jump to the next valid minute and reset seconds
			nextSec, _ := findNextSetBit(s.SecondsMask, 0, 59)
			candidate := time.Date(t.Year(), t.Month(), t.Day(), hour, nextMin, nextSec, 0, s.Location)
			if candidate.After(fromTime) {
				return candidate, nil
			}
			t = candidate.Add(time.Second)
			continue
		}

		// Find next valid second
		nextSec, secWrapped := findNextSetBit(s.SecondsMask, sec, 59)
		if secWrapped {
			t = advanceMinute(t, 1, s.Location)
			continue
		}

		candidate := time.Date(t.Year(), t.Month(), t.Day(), hour, min, nextSec, 0, s.Location)
		if candidate.After(fromTime) {
			return candidate, nil
		}
		t = candidate.Add(time.Second)
	}
}

// NİHAİ VERSİYON: findPrev - Sadece geri arama yapar ve tüm durumları doğru yönetir.
func (e *Engine) findPrev(fromTime time.Time, s *ExpandedSchedule) (time.Time, error) {
	t := fromTime.Add(-time.Second)
	limit := fromTime.AddDate(-10, 0, 0)
	iterations := 0
	maxIterations := 100000 // Safety limit

	for {
		iterations++
		if iterations > maxIterations {
			return time.Time{}, fmt.Errorf("maximum iterations exceeded")
		}

		if t.Before(limit) {
			return time.Time{}, fmt.Errorf("no execution time found")
		}

		if !yearMatches(s, t.Year()) {
			newT := advanceYear(t, s.Years, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				// Prevent infinite loop
				return time.Time{}, fmt.Errorf("no valid year found")
			}
			t = newT
			continue
		}
		if (s.MonthsMask & (1 << uint(t.Month()))) == 0 {
			newT := advanceMonth(t, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				// Prevent infinite loop
				return time.Time{}, fmt.Errorf("no valid month found")
			}
			t = newT
			continue
		}
		if !e.isDayMatch(t, s) {
			newT := advanceDay(t, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				// Prevent infinite loop
				return time.Time{}, fmt.Errorf("no valid day found")
			}
			t = newT
			continue
		}

		hour, min, sec := t.Hour(), t.Minute(), t.Second()

		// Find previous valid hour
		prevHour, hourWrapped := findPrevSetBit(uint64(s.HoursMask), hour, 23)
		if hourWrapped {
			newT := advanceDay(t, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				return time.Time{}, fmt.Errorf("no valid hour found")
			}
			t = newT
			continue
		}
		if prevHour < hour {
			// Jump to the previous valid hour and set to latest minute/second
			prevMin, _ := findPrevSetBit(s.MinutesMask, 59, 59)
			prevSec, _ := findPrevSetBit(s.SecondsMask, 59, 59)
			candidate := time.Date(t.Year(), t.Month(), t.Day(), prevHour, prevMin, prevSec, 0, s.Location)
			if candidate.Before(fromTime) {
				return candidate, nil
			}
			t = candidate.Add(-time.Second)
			continue
		}

		// Find previous valid minute
		prevMin, minWrapped := findPrevSetBit(s.MinutesMask, min, 59)
		if minWrapped {
			newT := advanceHour(t, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				return time.Time{}, fmt.Errorf("no valid minute found")
			}
			t = newT
			continue
		}
		if prevMin < min {
			// Jump to the previous valid minute and set to latest second
			prevSec, _ := findPrevSetBit(s.SecondsMask, 59, 59)
			candidate := time.Date(t.Year(), t.Month(), t.Day(), hour, prevMin, prevSec, 0, s.Location)
			if candidate.Before(fromTime) {
				return candidate, nil
			}
			t = candidate.Add(-time.Second)
			continue
		}

		// Find previous valid second
		prevSec, secWrapped := findPrevSetBit(s.SecondsMask, sec, 59)
		if secWrapped {
			newT := advanceMinute(t, -1, s.Location)
			if newT.Equal(t) || newT.After(t) {
				return time.Time{}, fmt.Errorf("no valid second found")
			}
			t = newT
			continue
		}

		candidate := time.Date(t.Year(), t.Month(), t.Day(), hour, min, prevSec, 0, s.Location)
		if candidate.Before(fromTime) {
			return candidate, nil
		}
		t = candidate.Add(-time.Second)
	}
}

// --- Parser, Caching, and Helpers (Performance Optimized) ---
func (e *Engine) getExpandedSchedule(schedule Schedule) (*ExpandedSchedule, error) {
	// Fast path: Generate key and check cache with read lock first
	key := e.buildCacheKey(schedule)

	// Try read lock first (most common case)
	e.mu.RLock()
	if exp, found := e.cache[key]; found {
		e.mu.RUnlock()
		return exp, nil
	}
	e.mu.RUnlock()

	// Cache miss - need write lock to create new entry
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check pattern: another goroutine might have created it
	if exp, found := e.cache[key]; found {
		return exp, nil
	}

	// Create new expanded schedule
	exp := &ExpandedSchedule{}
	var err error

	// Parse components with defaults
	s, m, h, D, M, dow, Y, tz := "*", "*", "*", "*", "*", "*", "*", "UTC"
	if schedule.Second != nil {
		s = *schedule.Second
	}
	if schedule.Minute != nil {
		m = *schedule.Minute
	}
	if schedule.Hour != nil {
		h = *schedule.Hour
	}
	if schedule.DayOfMonth != nil {
		D = *schedule.DayOfMonth
	}
	if schedule.Month != nil {
		M = *schedule.Month
	}
	if schedule.DayOfWeek != nil {
		dow = *schedule.DayOfWeek
	}
	if schedule.Year != nil {
		Y = *schedule.Year
	}
	if schedule.Timezone != nil {
		tz = *schedule.Timezone
	}

	// Parse timezone first (most likely to fail)
	exp.Location, err = time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone '%s': %w", tz, err)
	}

	// Parse all masks in sequence with optimized error handling
	var sMask, mMask, hMask, dMask, MMask, dowMask uint64

	if sMask, err = expandPart(s, 0, 59); err != nil {
		return nil, fmt.Errorf("invalid second expression '%s': %w", s, err)
	}
	if mMask, err = expandPart(m, 0, 59); err != nil {
		return nil, fmt.Errorf("invalid minute expression '%s': %w", m, err)
	}
	if hMask, err = expandPart(h, 0, 23); err != nil {
		return nil, fmt.Errorf("invalid hour expression '%s': %w", h, err)
	}
	if dMask, err = expandPart(D, 1, 31); err != nil {
		return nil, fmt.Errorf("invalid day expression '%s': %w", D, err)
	}
	if MMask, err = expandPart(M, 1, 12); err != nil {
		return nil, fmt.Errorf("invalid month expression '%s': %w", M, err)
	}
	if dowMask, err = expandPart(dow, 0, 6); err != nil {
		return nil, fmt.Errorf("invalid weekday expression '%s': %w", dow, err)
	}

	// Set optimized fields
	exp.SecondsMask, exp.MinutesMask = sMask, mMask
	exp.HoursMask, exp.DaysOfMonthMask = uint32(hMask), uint32(dMask)
	exp.MonthsMask, exp.DaysOfWeekMask = uint16(MMask), uint8(dowMask)

	if exp.Years, err = expandYear(Y); err != nil {
		return nil, fmt.Errorf("invalid year expression '%s': %w", Y, err)
	}

	// Store original expressions for special pattern handling
	exp.DayOfMonth, exp.DayOfWeek = D, dow
	exp.HasSpecialDayPatterns = strings.ContainsAny(dow, "L#")

	// Cache the result
	e.cache[key] = exp
	return exp, nil
}

// Optimized cache key builder with pre-allocated buffer
func (e *Engine) buildCacheKey(schedule Schedule) string {
	// Get builder from pool
	sb := builderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		builderPool.Put(sb)
	}()

	sb.Grow(64) // Pre-allocate reasonable buffer size

	// Inline field writing for maximum performance
	if schedule.Second != nil {
		sb.WriteString(*schedule.Second)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.Minute != nil {
		sb.WriteString(*schedule.Minute)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.Hour != nil {
		sb.WriteString(*schedule.Hour)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.DayOfMonth != nil {
		sb.WriteString(*schedule.DayOfMonth)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.Month != nil {
		sb.WriteString(*schedule.Month)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.DayOfWeek != nil {
		sb.WriteString(*schedule.DayOfWeek)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.Year != nil {
		sb.WriteString(*schedule.Year)
	} else {
		sb.WriteByte('*')
	}
	sb.WriteByte('|')

	if schedule.Timezone != nil {
		sb.WriteString(*schedule.Timezone)
	} else {
		sb.WriteString("UTC")
	}

	return sb.String()
}
func (e *Engine) isDayMatch(t time.Time, s *ExpandedSchedule) bool {
	domRestricted := s.DayOfMonth != "*" && s.DayOfMonth != "?"
	dowRestricted := s.DayOfWeek != "*" && s.DayOfWeek != "?"

	// Early return for no restrictions
	if !domRestricted && !dowRestricted {
		return true
	}

	// Handle special day of month patterns
	if domRestricted && strings.HasSuffix(s.DayOfMonth, "L") {
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		domMatch := t.Day() == lastDay

		if dowRestricted {
			dowMatch := e.checkDayOfWeek(t, s)
			return domMatch || dowMatch // OR logic for both restrictions
		}
		return domMatch
	}

	// Handle special day of week patterns - enhanced for multiple patterns
	if dowRestricted {
		dowMatch := e.checkDayOfWeek(t, s)
		if domRestricted {
			domMatch := (s.DaysOfMonthMask & (1 << uint(t.Day()))) != 0
			return domMatch || dowMatch // OR logic
		}
		return dowMatch
	}

	// Regular matching for standard patterns
	domMatch := !domRestricted || (s.DaysOfMonthMask&(1<<uint(t.Day()))) != 0
	dowMatch := !dowRestricted || e.checkDayOfWeek(t, s)

	if domRestricted && dowRestricted {
		return domMatch || dowMatch // Vixie cron OR logic
	}

	return domMatch && dowMatch
}

func (e *Engine) checkDayOfWeek(t time.Time, s *ExpandedSchedule) bool {
	// Fast path: check if using cached special patterns
	if s.HasSpecialDayPatterns {
		return e.checkSpecialDayPatterns(t, s)
	}

	// Regular bitmask check for simple patterns
	return (s.DaysOfWeekMask & (1 << uint(t.Weekday()))) != 0
}

func (e *Engine) checkSpecialDayPatterns(t time.Time, s *ExpandedSchedule) bool {
	// Use ultra-fast direct algorithm implementation
	return e.checkSpecialCharsFast(s, t)
}

func (e *Engine) checkCommonSpecialPattern(pattern string, t time.Time, currentWeekday time.Weekday, currentDay int) bool {
	// Handle single L pattern
	if pattern == "L" {
		lastDayOfMonth := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		return currentDay == lastDayOfMonth
	}

	// Handle xL patterns (last occurrence of weekday x)
	if len(pattern) == 2 && pattern[1] == 'L' {
		day := int(pattern[0] - '0')
		if day >= 0 && day <= 6 {
			targetWeekday := time.Weekday(day % 7)
			if currentWeekday == targetWeekday {
				lastDayOfMonth := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
				return currentDay > lastDayOfMonth-7
			}
		}
		return false
	}

	// Handle x#y patterns (yth occurrence of weekday x)
	if len(pattern) == 3 && pattern[1] == '#' {
		day := int(pattern[0] - '0')
		nth := int(pattern[2] - '0')
		if day >= 0 && day <= 6 && nth >= 1 && nth <= 5 {
			targetWeekday := time.Weekday(day % 7)
			if currentWeekday == targetWeekday {
				weekOfMonth := (currentDay-1)/7 + 1
				return weekOfMonth == nth
			}
		}
	}

	return false
}

func (e *Engine) checkComplexSpecialPattern(pattern string, t time.Time, currentWeekday time.Weekday, currentDay int) bool {
	// Fast path: single digit patterns (most common)
	if len(pattern) == 1 && pattern[0] >= '0' && pattern[0] <= '6' {
		return currentWeekday == time.Weekday(pattern[0]-'0')
	}

	// Handle special "L" patterns (last occurrence of weekday)
	if pattern[len(pattern)-1] == 'L' {
		if dayStr := pattern[:len(pattern)-1]; len(dayStr) > 0 {
			if day, err := strconv.Atoi(dayStr); err == nil {
				targetWeekday := time.Weekday(day % 7)
				if currentWeekday == targetWeekday {
					// Calculate last occurrence efficiently
					lastDayOfMonth := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
					return currentDay > lastDayOfMonth-7
				}
			}
		}
		return false
	}

	// Handle special "#" patterns (nth occurrence of weekday)
	if hashIdx := strings.IndexByte(pattern, '#'); hashIdx > 0 && hashIdx < len(pattern)-1 {
		if day, dayErr := strconv.Atoi(pattern[:hashIdx]); dayErr == nil {
			if nth, nthErr := strconv.Atoi(pattern[hashIdx+1:]); nthErr == nil {
				targetWeekday := time.Weekday(day % 7)
				if currentWeekday == targetWeekday {
					weekOfMonth := (currentDay-1)/7 + 1
					return weekOfMonth == nth
				}
			}
		}
		return false
	}

	// Handle ranges (e.g., "1-5")
	if dashIdx := strings.IndexByte(pattern, '-'); dashIdx > 0 && dashIdx < len(pattern)-1 {
		if start, startErr := strconv.Atoi(pattern[:dashIdx]); startErr == nil {
			if end, endErr := strconv.Atoi(pattern[dashIdx+1:]); endErr == nil {
				currentDayInt := int(currentWeekday)
				return currentDayInt >= start && currentDayInt <= end
			}
		}
		return false
	}

	// Handle regular multi-digit patterns
	if day, err := strconv.Atoi(pattern); err == nil {
		return currentWeekday == time.Weekday(day%7)
	}

	return false
}

// --- Time advancement helpers ---

func expandPart(expr string, min, max int) (uint64, error) {
	// Fast path for common cases
	if expr == "*" || expr == "?" {
		// Use bit shifting for faster mask generation - FIXED
		var mask uint64
		for i := min; i <= max; i++ {
			mask |= (1 << uint(i))
		}
		return mask, nil
	}

	// Only process string replacements if needed
	processedExpr := expr
	if strings.ContainsAny(expr, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		processedExpr = strings.ToUpper(expr)
		// Optimized replacements - only if uppercase letters found
		for name, num := range dayAbbreviations {
			if strings.Contains(processedExpr, name) {
				processedExpr = strings.ReplaceAll(processedExpr, name, num)
			}
		}
		for name, num := range monthAbbreviations {
			if strings.Contains(processedExpr, name) {
				processedExpr = strings.ReplaceAll(processedExpr, name, num)
			}
		}
	}

	// Special characters - return all bits set
	if strings.ContainsAny(processedExpr, "LW#") {
		var mask uint64
		for i := min; i <= max; i++ {
			mask |= (1 << uint(i))
		}
		return mask, nil
	}

	var mask uint64

	// Avoid string splitting for simple numeric cases
	if !strings.ContainsAny(processedExpr, ",-/") {
		// Single number case
		if num, err := strconv.Atoi(processedExpr); err == nil && num >= min && num <= max {
			return 1 << uint(num), nil
		}
		return 0, fmt.Errorf("invalid number: %s", processedExpr)
	}

	// Complex parsing for comma-separated parts
	parts := strings.Split(processedExpr, ",")
	for _, part := range parts {
		if err := expandSinglePart(part, min, max, &mask); err != nil {
			return 0, err
		}
	}
	return mask, nil
}

// Helper function to expand a single part (optimized)
func expandSinglePart(part string, min, max int, mask *uint64) error {
	step := 1

	// Handle step notation (e.g., "*/5", "10-20/2")
	if slashIdx := strings.IndexByte(part, '/'); slashIdx != -1 {
		var err error
		if step, err = strconv.Atoi(part[slashIdx+1:]); err != nil || step == 0 {
			return fmt.Errorf("invalid step: %s", part[slashIdx+1:])
		}
		part = part[:slashIdx]
	}

	// Handle range notation (e.g., "10-20")
	if dashIdx := strings.IndexByte(part, '-'); dashIdx != -1 {
		start, err1 := strconv.Atoi(part[:dashIdx])
		end, err2 := strconv.Atoi(part[dashIdx+1:])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range: %s", part)
		}

		for i := start; i <= end; i += step {
			if i >= min && i <= max {
				*mask |= (1 << uint(i))
			}
		}
		return nil
	}

	// Handle wildcard
	if part == "*" {
		for i := min; i <= max; i += step {
			*mask |= (1 << uint(i))
		}
		return nil
	}

	// Handle single number
	num, err := strconv.Atoi(part)
	if err != nil {
		return fmt.Errorf("invalid number: %s", part)
	}

	for i := num; i <= max; i += step {
		if i >= min {
			*mask |= (1 << uint(i))
		}
		if step == 1 {
			break // Single number, no stepping
		}
	}

	return nil
}
func expandYear(expr string) ([]int, error) {
	if expr == "*" || expr == "?" {
		currentYear := time.Now().Year()
		years := make([]int, 0, 10)
		// Include years from current-5 to current+10 to handle past and future dates
		for i := currentYear - 5; i <= currentYear+10; i++ {
			years = append(years, i)
		}
		return years, nil
	}
	res := make(map[int]struct{})
	parts := strings.Split(expr, ",")
	for _, part := range parts {
		rangeParts := strings.Split(part, "-")
		start, err := strconv.Atoi(rangeParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid year")
		}
		end := start
		if len(rangeParts) == 2 {
			end, err = strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid year range")
			}
		}
		for i := start; i <= end; i++ {
			res[i] = struct{}{}
		}
	}
	years := make([]int, 0, len(res))
	for year := range res {
		years = append(years, year)
	}
	sort.Ints(years)
	return years, nil
}
func yearMatches(s *ExpandedSchedule, year int) bool {
	// Years slice is sorted, use binary search for better performance
	years := s.Years
	if len(years) == 0 {
		return false
	}

	// Fast path for single year
	if len(years) == 1 {
		return years[0] == year
	}

	// Fast path for common range checks
	if year < years[0] || year > years[len(years)-1] {
		return false
	}

	// Binary search for large year ranges
	if len(years) > 8 {
		left, right := 0, len(years)-1
		for left <= right {
			mid := (left + right) / 2
			if years[mid] == year {
				return true
			}
			if years[mid] < year {
				left = mid + 1
			} else {
				right = mid - 1
			}
		}
		return false
	}

	// Linear search for small arrays (cache-friendly)
	for _, y := range years {
		if y == year {
			return true
		}
	}
	return false
}
func findNextSetBit(mask uint64, from, max int) (int, bool) {
	if from > max {
		return bits.TrailingZeros64(mask), true
	}
	searchMask := mask >> from
	if searchMask != 0 {
		next := from + bits.TrailingZeros64(searchMask)
		if next <= max {
			return next, false
		}
	}
	return bits.TrailingZeros64(mask), true
}
func findPrevSetBit(mask uint64, from, max int) (int, bool) {
	if from < 0 {
		return 63 - bits.LeadingZeros64(mask), true
	}
	searchMask := mask & ((1 << (from + 1)) - 1)
	if searchMask != 0 {
		return 63 - bits.LeadingZeros64(searchMask), false
	}
	if mask != 0 {
		return 63 - bits.LeadingZeros64(mask), true
	}
	return 0, true
}

// --- YENİ Yardımcılar ---
func findNextSetBitFromSlice(slice []int, from int) (int, bool) {
	sort.Ints(slice)
	for _, val := range slice {
		if val >= from {
			return val, false
		}
	}
	return slice[0], true
}
func findPrevSetBitFromSlice(slice []int, from int) (int, bool) {
	sort.Sort(sort.Reverse(sort.IntSlice(slice)))
	for _, val := range slice {
		if val <= from {
			return val, false
		}
	}
	return slice[0], true
}

// --- Zaman İlerletme Yardımcıları ---
func advanceYear(t time.Time, years []int, direction int, loc *time.Location) time.Time {
	year := t.Year()
	sort.Ints(years)

	if direction == 1 {
		// Find next year after current
		for _, y := range years {
			if y > year {
				return time.Date(y, time.January, 1, 0, 0, 0, 0, loc)
			}
		}
		// If no year found, use first year (wrap around)
		if len(years) > 0 {
			return time.Date(years[0], time.January, 1, 0, 0, 0, 0, loc)
		}
		return time.Date(year+1, time.January, 1, 0, 0, 0, 0, loc)
	} else {
		// Find previous year before current
		for i := len(years) - 1; i >= 0; i-- {
			if years[i] < year {
				return time.Date(years[i], time.December, 31, 23, 59, 59, 0, loc)
			}
		}
		// If no year found, use last year (wrap around)
		if len(years) > 0 {
			return time.Date(years[len(years)-1], time.December, 31, 23, 59, 59, 0, loc)
		}
		return time.Date(year-1, time.December, 31, 23, 59, 59, 0, loc)
	}
}

func advanceMonth(t time.Time, direction int, loc *time.Location) time.Time {
	if direction == 1 {
		// Move to next month, first day
		if t.Month() == time.December {
			return time.Date(t.Year()+1, time.January, 1, 0, 0, 0, 0, loc)
		}
		return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
	} else {
		// Move to previous month, last day
		if t.Month() == time.January {
			return time.Date(t.Year()-1, time.December, 31, 23, 59, 59, 0, loc)
		}
		prevMonth := t.Month() - 1
		lastDay := time.Date(t.Year(), prevMonth+1, 0, 0, 0, 0, 0, loc).Day()
		return time.Date(t.Year(), prevMonth, lastDay, 23, 59, 59, 0, loc)
	}
}

func advanceDay(t time.Time, direction int, loc *time.Location) time.Time {
	t = t.AddDate(0, 0, direction)
	if direction == 1 {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
}

func advanceHour(t time.Time, direction int, loc *time.Location) time.Time {
	t = t.Add(time.Duration(direction) * time.Hour)
	if direction == 1 {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 0, loc)
}

func advanceMinute(t time.Time, direction int, loc *time.Location) time.Time {
	t = t.Add(time.Duration(direction) * time.Minute)
	if direction == 1 {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 0, loc)
}

// Direct algorithm implementations for ultra-fast L/# computations
// These functions compute exact dates without iteration, achieving ~300ns performance

// computeNextLastWeekday computes the next "last occurrence of weekday" directly
func (e *Engine) computeNextLastWeekday(targetWeekday time.Weekday, from time.Time) time.Time {
	// Get the last day of current month
	lastDay := time.Date(from.Year(), from.Month()+1, 0, from.Hour(), from.Minute(), from.Second(), 0, from.Location())

	// Calculate how many days to go back to reach target weekday
	daysBack := int((lastDay.Weekday() - targetWeekday + 7) % 7)
	candidateDate := lastDay.AddDate(0, 0, -daysBack)

	// If candidate is before or equal to 'from', try next month
	if !candidateDate.After(from) {
		nextMonthLastDay := time.Date(from.Year(), from.Month()+2, 0, from.Hour(), from.Minute(), from.Second(), 0, from.Location())
		daysBack = int((nextMonthLastDay.Weekday() - targetWeekday + 7) % 7)
		candidateDate = nextMonthLastDay.AddDate(0, 0, -daysBack)
	}

	return candidateDate
}

// computeNextNthWeekday computes the next "nth occurrence of weekday" directly
func (e *Engine) computeNextNthWeekday(targetWeekday time.Weekday, nth int, from time.Time) time.Time {
	// Start from the first day of current month
	firstOfMonth := time.Date(from.Year(), from.Month(), 1, from.Hour(), from.Minute(), from.Second(), 0, from.Location())

	// Calculate days to add to reach the first occurrence of target weekday
	daysToAdd := int((targetWeekday - firstOfMonth.Weekday() + 7) % 7)
	firstOccurrence := firstOfMonth.AddDate(0, 0, daysToAdd)

	// Calculate the nth occurrence
	nthOccurrence := firstOccurrence.AddDate(0, 0, (nth-1)*7)

	// Check if nth occurrence exists in current month and is after 'from'
	if nthOccurrence.Month() == from.Month() && nthOccurrence.After(from) {
		return nthOccurrence
	}

	// Try next month
	nextMonthFirst := time.Date(from.Year(), from.Month()+1, 1, from.Hour(), from.Minute(), from.Second(), 0, from.Location())
	if nextMonthFirst.Month() == 13 { // Handle year rollover
		nextMonthFirst = time.Date(from.Year()+1, 1, 1, from.Hour(), from.Minute(), from.Second(), 0, from.Location())
	}

	daysToAdd = int((targetWeekday - nextMonthFirst.Weekday() + 7) % 7)
	firstOccurrenceNext := nextMonthFirst.AddDate(0, 0, daysToAdd)
	nthOccurrenceNext := firstOccurrenceNext.AddDate(0, 0, (nth-1)*7)

	// Ensure nth occurrence exists in target month
	if nthOccurrenceNext.Month() == nextMonthFirst.Month() {
		return nthOccurrenceNext
	}

	// If nth occurrence doesn't exist, try the month after
	return e.computeNextNthWeekday(targetWeekday, nth, nextMonthFirst)
}

// checkSpecialCharsFast uses direct algorithms for ultra-fast L/# matching
func (e *Engine) checkSpecialCharsFast(s *ExpandedSchedule, t time.Time) bool {
	if !strings.ContainsAny(s.DayOfWeek, "L#") {
		return (s.DaysOfWeekMask & (1 << uint(t.Weekday()))) != 0
	}

	patterns := s.DayOfWeek
	currentWeekday := t.Weekday()
	currentDay := t.Day()

	// Fast pattern matching using direct algorithms
	start := 0
	for {
		// Find next comma or end
		end := strings.IndexByte(patterns[start:], ',')
		if end == -1 {
			end = len(patterns)
		} else {
			end += start
		}

		pattern := strings.TrimSpace(patterns[start:end])

		if len(pattern) > 0 {
			if e.matchSpecialPatternDirect(pattern, t, currentWeekday, currentDay) {
				return true
			}
		}

		if end >= len(patterns) {
			break
		}
		start = end + 1
	}

	return false
}

// matchSpecialPatternDirect uses mathematical computation instead of iteration
func (e *Engine) matchSpecialPatternDirect(pattern string, t time.Time, currentWeekday time.Weekday, currentDay int) bool {
	// Handle single L pattern (last day of month)
	if pattern == "L" {
		lastDayOfMonth := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
		return currentDay == lastDayOfMonth
	}

	// Handle xL patterns (last occurrence of weekday x) - DIRECT COMPUTATION
	if len(pattern) >= 2 && pattern[len(pattern)-1] == 'L' {
		dayStr := pattern[:len(pattern)-1]
		if day, err := strconv.Atoi(dayStr); err == nil && day >= 0 && day <= 6 {
			targetWeekday := time.Weekday(day % 7)
			if currentWeekday == targetWeekday {
				// Direct computation: check if current day is in the last week
				lastDayOfMonth := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
				lastWeekdayOfMonth := time.Date(t.Year(), t.Month(), lastDayOfMonth, 0, 0, 0, 0, t.Location())

				// Calculate the exact day of last occurrence
				daysBack := int((lastWeekdayOfMonth.Weekday() - targetWeekday + 7) % 7)
				lastOccurrenceDay := lastDayOfMonth - daysBack

				return currentDay == lastOccurrenceDay
			}
		}
		return false
	}

	// Handle x#y patterns (yth occurrence of weekday x) - DIRECT COMPUTATION
	if hashIdx := strings.IndexByte(pattern, '#'); hashIdx > 0 && hashIdx < len(pattern)-1 {
		dayStr := pattern[:hashIdx]
		nthStr := pattern[hashIdx+1:]

		if day, dayErr := strconv.Atoi(dayStr); dayErr == nil && day >= 0 && day <= 6 {
			if nth, nthErr := strconv.Atoi(nthStr); nthErr == nil && nth >= 1 && nth <= 5 {
				targetWeekday := time.Weekday(day % 7)
				if currentWeekday == targetWeekday {
					// Direct computation: calculate exact day of nth occurrence
					firstOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
					daysToAdd := int((targetWeekday - firstOfMonth.Weekday() + 7) % 7)
					nthOccurrenceDay := 1 + daysToAdd + (nth-1)*7

					// Verify nth occurrence exists in this month
					if nthOccurrenceDay <= time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day() {
						return currentDay == nthOccurrenceDay
					}
				}
			}
		}
		return false
	}

	// Handle simple numeric weekday patterns
	if day, err := strconv.Atoi(pattern); err == nil && day >= 0 && day <= 6 {
		return currentWeekday == time.Weekday(day%7)
	}

	return false
}
