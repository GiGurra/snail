package snail_slice

import (
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"testing"
)

func TestDiscardFirstN(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	expected := append([]int{}, slice[3:]...)
	actual := DiscardFirstN(slice, 3)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardFirstN_many(t *testing.T) {
	slice := lo.Range(1000)
	expected := append([]int{}, slice[565:]...)
	actual := DiscardFirstN(slice, 565)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardFirstN_all(t *testing.T) {
	slice := lo.Range(1000)
	expected := append([]int{}, slice[1000:]...)
	actual := DiscardFirstN(slice, 1000)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardFirstN_one(t *testing.T) {
	slice := lo.Range(10)
	expected := append([]int{}, slice[1:]...)
	actual := DiscardFirstN(slice, 1)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardFirstN_none(t *testing.T) {
	slice := lo.Range(1000)
	expected := slice[0:]
	actual := DiscardFirstN(slice, 0)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardAt(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	expected := []int{1, 2, 4, 5}
	actual := DiscardAt(slice, 2)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardAt_head(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	expected := []int{2, 3, 4, 5}
	actual := DiscardAt(slice, 0)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestDiscardAt_tail(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	expected := []int{1, 2, 3, 4}
	actual := DiscardAt(slice, 4)
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
