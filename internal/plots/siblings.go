package plots

type Neighbour struct {
	PublicID   string
	Name       string
	MemberName string
	AreaHa     float64
}

type Neighbours struct {
	Position int
	Total    int
	Previous *Neighbour
	Next     *Neighbour
	Others   []Neighbour
}

func NeighboursOf(list []Neighbour, publicID string) Neighbours {
	index := -1
	for i, plot := range list {
		if plot.PublicID == publicID {
			index = i
			break
		}
	}

	if index < 0 {
		return Neighbours{
			Position: 0,
			Total:    len(list),
			Others:   append([]Neighbour{}, list...),
		}
	}

	neighbours := Neighbours{
		Position: index + 1,
		Total:    len(list),
		Others:   []Neighbour{},
	}

	if index > 0 {
		previous := list[index-1]
		neighbours.Previous = &previous
	}
	if index < len(list)-1 {
		next := list[index+1]
		neighbours.Next = &next
	}

	for i, plot := range list {
		if i != index {
			neighbours.Others = append(neighbours.Others, plot)
		}
	}

	return neighbours
}
