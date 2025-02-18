// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import "bytes"

type lItem struct {
	LTime   LamportTime
	ID      uint32
	Payload []byte
}

func (l *lItem) Equal(item *lItem) bool {
	if l.LTime != item.LTime {
		return false
	}
	if l.Payload != nil {
		return bytes.Equal(l.Payload, item.Payload) // for action
	}
	return l.ID == item.ID // for query
}

type lGroupItem struct {
	Items []*lItem
	LTime LamportTime
}

func (g *lGroupItem) has(item *lItem) bool {
	for _, i := range g.Items {
		if i.Equal(item) {
			return true
		}
	}
	return false
}

func (g *lGroupItem) add(item *lItem) {
	g.Items = append(g.Items, item)
}

type lBuffer []*lGroupItem

func (b *lBuffer) len() LamportTime {
	return LamportTime(len(*b))
}

func (b *lBuffer) isTooOld(currentTime LamportTime, item *lItem) bool {
	if currentTime <= b.len() {
		return false
	}
	return item.LTime < currentTime-b.len()
}

func (b *lBuffer) isLTimeNew(t LamportTime) bool {
	idx := t % b.len()
	group := (*b)[idx]
	return group == nil || group.LTime < t
}

func (b *lBuffer) addNewLTime(item *lItem) {
	idx := item.LTime % b.len()
	(*b)[idx] = &lGroupItem{
		Items: []*lItem{item},
		LTime: item.LTime,
	}
}

func (b *lBuffer) addItem(currentTime LamportTime, item *lItem) bool {
	if b.isTooOld(currentTime, item) {
		return false
	}
	if b.isLTimeNew(item.LTime) {
		b.addNewLTime(item)
		return true
	}
	idx := item.LTime % b.len()
	group := (*b)[idx]
	if group.has(item) {
		return false
	}
	group.add(item)
	return true
}
