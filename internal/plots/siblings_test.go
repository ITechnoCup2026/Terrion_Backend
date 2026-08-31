package plots_test

import (
	"testing"

	"terrion-backend/internal/plots"
)

func neighbour(publicID string) plots.Neighbour {
	return plots.Neighbour{
		PublicID: publicID, Name: publicID, MemberName: "Anggota", AreaHa: 1,
	}
}

var threePlots = []plots.Neighbour{neighbour("a"), neighbour("b"), neighbour("c")}

func publicIDsOf(list []plots.Neighbour) []string {
	ids := make([]string, len(list))
	for i, plot := range list {
		ids[i] = plot.PublicID
	}
	return ids
}

func TestNeighboursOfReportsThePositionInTheCooperative(t *testing.T) {
	neighbours := plots.NeighboursOf(threePlots, "b")

	if neighbours.Position != 2 || neighbours.Total != 3 {
		t.Errorf("position %d of %d, want 2 of 3", neighbours.Position, neighbours.Total)
	}
}

func TestNeighboursOfGivesBothNeighboursInTheMiddle(t *testing.T) {
	neighbours := plots.NeighboursOf(threePlots, "b")

	if neighbours.Previous == nil || neighbours.Previous.PublicID != "a" {
		t.Errorf("Previous = %v, want a", neighbours.Previous)
	}
	if neighbours.Next == nil || neighbours.Next.PublicID != "c" {
		t.Errorf("Next = %v, want c", neighbours.Next)
	}
}

func TestNeighboursOfDoesNotWrapAtTheEnds(t *testing.T) {
	first := plots.NeighboursOf(threePlots, "a")
	if first.Previous != nil {
		t.Errorf("Previous at the first plot = %v, want nil: the ends are ends", first.Previous)
	}
	if first.Next == nil || first.Next.PublicID != "b" {
		t.Errorf("Next = %v, want b", first.Next)
	}

	last := plots.NeighboursOf(threePlots, "c")
	if last.Previous == nil || last.Previous.PublicID != "b" {
		t.Errorf("Previous = %v, want b", last.Previous)
	}
	if last.Next != nil {
		t.Errorf("Next at the last plot = %v, want nil", last.Next)
	}
}

func TestNeighboursOfListsEveryOtherPlotInOrder(t *testing.T) {
	others := publicIDsOf(plots.NeighboursOf(threePlots, "b").Others)

	if len(others) != 2 || others[0] != "a" || others[1] != "c" {
		t.Errorf("Others = %v, want [a c]", others)
	}
}

func TestNeighboursOfACooperativeWithOnePlotOffersNothing(t *testing.T) {
	neighbours := plots.NeighboursOf([]plots.Neighbour{neighbour("only")}, "only")

	if neighbours.Position != 1 || neighbours.Total != 1 {
		t.Errorf("position %d of %d, want 1 of 1", neighbours.Position, neighbours.Total)
	}
	if neighbours.Previous != nil || neighbours.Next != nil {
		t.Errorf("neighbours = %+v, want none either side", neighbours)
	}
	if len(neighbours.Others) != 0 {
		t.Errorf("Others = %v, want empty", neighbours.Others)
	}
}

func TestNeighboursOfSurvivesAPlotMissingFromItsOwnList(t *testing.T) {
	neighbours := plots.NeighboursOf(threePlots, "ghost")

	if neighbours.Position != 0 || neighbours.Total != 3 {
		t.Errorf("position %d of %d, want 0 of 3", neighbours.Position, neighbours.Total)
	}
	if neighbours.Previous != nil || neighbours.Next != nil {
		t.Errorf("neighbours = %+v, want none either side", neighbours)
	}
	if len(neighbours.Others) != 3 {
		t.Errorf("Others = %v, want the whole list", publicIDsOf(neighbours.Others))
	}
}

func TestNeighboursOfAnEmptyListIsEmpty(t *testing.T) {
	neighbours := plots.NeighboursOf(nil, "a")

	if neighbours.Position != 0 || neighbours.Total != 0 || len(neighbours.Others) != 0 {
		t.Errorf("neighbours = %+v, want everything empty", neighbours)
	}
}

func TestNeighboursOfDoesNotMutateItsInput(t *testing.T) {
	before := publicIDsOf(threePlots)

	plots.NeighboursOf(threePlots, "b")

	after := publicIDsOf(threePlots)
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("input changed from %v to %v", before, after)
			break
		}
	}
}
