package ui

// VisibleRange returns the half-open item range [start, end) that intersects a
// scroll viewport. itemExtent is the fixed row/card size along the scroll axis;
// overscan includes nearby items so fast scrolling does not reveal blanks.
func VisibleRange(offset, viewport, itemExtent float64, count, overscan int) (start, end int) {
	if count <= 0 || itemExtent <= 0 || viewport <= 0 {
		return 0, 0
	}
	start = int(offset / itemExtent)
	end = int((offset+viewport)/itemExtent) + 1
	start -= overscan
	end += overscan
	if start < 0 {
		start = 0
	}
	if end > count {
		end = count
	}
	if end < start {
		end = start
	}
	return start, end
}
