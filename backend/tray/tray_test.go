package tray

import (
	"bytes"
	"testing"

	"slipstream/backend/statemachine"
)

func TestIconForPriority(t *testing.T) {
	cases := []struct {
		name string
		s    statemachine.Status
		want []byte
	}{
		{"idle", statemachine.Status{State: statemachine.StateIdle}, offIcon},
		{"fast active", statemachine.Status{State: statemachine.StateFastActive}, fastIcon},
		{"error", statemachine.Status{State: statemachine.StateError}, alertIcon},
		{
			"error takes priority over fast active",
			statemachine.Status{State: statemachine.StateError},
			alertIcon,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := iconFor(c.s); !bytes.Equal(got, c.want) {
				t.Errorf("iconFor(%+v) picked the wrong icon", c.s)
			}
		})
	}
}

func TestTooltipForNonEmpty(t *testing.T) {
	states := []statemachine.Status{
		{State: statemachine.StateIdle},
		{State: statemachine.StateFastActive},
		{State: statemachine.StateError},
	}
	for _, s := range states {
		if got := tooltipFor(s); got == "" {
			t.Errorf("tooltipFor(%+v) returned empty string", s)
		}
	}
}
