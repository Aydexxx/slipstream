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
		{"private active", statemachine.Status{State: statemachine.StatePrivateActive}, privateIcon},
		{"error", statemachine.Status{State: statemachine.StateError}, alertIcon},
		{
			"kill switch armed while connecting",
			statemachine.Status{State: statemachine.StatePrivateConnecting, KillSwitchArmed: true},
			alertIcon,
		},
		{
			"kill switch armed and connected is normal, not alert",
			statemachine.Status{State: statemachine.StatePrivateActive, KillSwitchArmed: true},
			privateIcon,
		},
		{
			"error takes priority over armed-and-connected",
			statemachine.Status{State: statemachine.StateError, KillSwitchArmed: true},
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
		{State: statemachine.StatePrivateActive},
		{State: statemachine.StateError},
		{State: statemachine.StatePrivateConnecting, KillSwitchArmed: true},
	}
	for _, s := range states {
		if got := tooltipFor(s); got == "" {
			t.Errorf("tooltipFor(%+v) returned empty string", s)
		}
	}
}
