package serf

type lItem interface {
	LTime() LamportTime
	Equal(lItem) bool
}

type lGroupItem struct {
	items []lItem
	LTime LamportTime
}

func (g *lGroupItem) has(item lItem) bool {
	for _, i := range g.items {
		if i.Equal(item) {
			return true
		}
	}
	return false
}

func (g *lGroupItem) add(item lItem) {
	g.items = append(g.items, item)
}

type lBuffer []*lGroupItem

func (b *lBuffer) len() LamportTime {
	return LamportTime(len(*b))
}

func (b *lBuffer) isTooOld(currentTime LamportTime, item lItem) bool {
	if currentTime <= b.len() {
		return false
	}
	return item.LTime() < currentTime-b.len()
}

func (b *lBuffer) isLTimeNew(t LamportTime) bool {
	idx := t % b.len()
	group := (*b)[idx]
	return group == nil || group.LTime < t
}

func (b *lBuffer) addNewLTime(item lItem) {
	idx := item.LTime() % b.len()
	(*b)[idx] = &lGroupItem{
		items: []lItem{item},
		LTime: item.LTime(),
	}
}
