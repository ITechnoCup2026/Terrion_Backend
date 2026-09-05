package aiclient

import "strconv"

type RefTable struct {
	forward map[string]string
	back    map[string]string
	counts  map[string]int
}

func NewRefTable() *RefTable {
	return &RefTable{
		forward: make(map[string]string),
		back:    make(map[string]string),
		counts:  make(map[string]int),
	}
}

func (t *RefTable) Plot(id string) string      { return t.assign("p", id) }
func (t *RefTable) Commodity(id string) string { return t.assign("k", id) }
func (t *RefTable) Variety(id string) string   { return t.assign("v", id) }

func (t *RefTable) Resolve(ref string) (string, bool) {
	id, ok := t.back[ref]
	return id, ok
}

func (t *RefTable) assign(prefix, id string) string {
	key := prefix + ":" + id
	if existing, ok := t.forward[key]; ok {
		return existing
	}

	t.counts[prefix]++
	ref := prefix + strconv.Itoa(t.counts[prefix])
	t.forward[key] = ref
	t.back[ref] = id

	return ref
}
