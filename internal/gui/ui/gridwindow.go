package ui

// What a preload dispatch warms is the span of tiles the grid asked about since
// the last dispatch, not the single tile that missed: one scroll step updates a
// whole row, and the miss may come from any tile in it. The range starts unset
// rather than at zero, so the first dispatch after a scroll aims at the tiles
// that are actually on screen instead of at the top of the folder.
type visibleRange struct {
	min int
	max int
	set bool
}

func (r *visibleRange) observe(id int) {
	if !r.set || id < r.min {
		r.min = id
	}
	if !r.set || id > r.max {
		r.max = id
	}
	r.set = true
}

func (r *visibleRange) reset() {
	*r = visibleRange{}
}

// hi < lo means there is nothing to warm - an empty folder, or no tile seen
// since the last dispatch.
func (r *visibleRange) bounds(count, buffer int) (lo, hi int) {
	if count <= 0 || !r.set {
		return 0, -1
	}
	return max(r.min-buffer, 0), min(r.max+buffer, count-1)
}
