//go:build windows

package main

import "testing"

func TestIsDescendant(t *testing.T) {
	parents := map[uint32]uint32{
		20: 10,
		30: 20,
		40: 99,
		50: 60,
		60: 50,
	}
	for _, test := range []struct {
		pid, root uint32
		want      bool
	}{
		{30, 10, true},
		{20, 10, true},
		{10, 10, true},
		{40, 10, false},
		{50, 10, false},
	} {
		if got := isDescendant(test.pid, test.root, parents); got != test.want {
			t.Errorf("isDescendant(%d, %d) = %v, want %v", test.pid, test.root, got, test.want)
		}
	}
}
