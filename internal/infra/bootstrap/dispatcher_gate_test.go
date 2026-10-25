package bootstrap

import "testing"

func TestDispatcherEnabled(t *testing.T) {
	for _, c := range []struct{ enabled, skip, want bool }{
		{true, false, true}, {true, true, false}, {false, false, false}, {false, true, false},
	} {
		var o Options
		o.Config.Notifications.Enabled = c.enabled
		o.SkipPurger = c.skip
		if got := dispatcherEnabled(o); got != c.want {
			t.Errorf("enabled=%v skip=%v got %v", c.enabled, c.skip, got)
		}
	}
}
