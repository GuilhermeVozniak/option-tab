package platform

import "testing"

func TestLauncherSpaceRequiresCorroboratedIntegralMetadata(t *testing.T) {
	good := `[{"Display Identifier":"display","Current Space":{"id64":6,"ManagedSpaceID":6,"type":0},"Spaces":[{"id64":6,"ManagedSpaceID":6,"type":0}]}]`
	for _, tt := range []struct {
		data string
		want bool
	}{{good, true}, {`[{"Display Identifier":"display","Current Space":{"id64":6,"ManagedSpaceID":6,"type":false},"Spaces":[{"id64":6,"ManagedSpaceID":6,"type":0}]}]`, false}, {`[{"Display Identifier":"display","Current Space":{"id64":6,"ManagedSpaceID":6,"type":0},"Spaces":[]}]`, false}, {`[{"Display Identifier":"display","Current Space":{"id64":6.5,"ManagedSpaceID":6.5,"type":0},"Spaces":[{"id64":6.5,"ManagedSpaceID":6.5,"type":0}]}]`, false}} {
		d := []LauncherDisplay{{UUID: "display"}}
		applyLauncherSpaces(d, []byte(tt.data))
		if (d[0].SpaceKind == "ordinary") != tt.want {
			t.Fatalf("unsafe classification: %+v", d)
		}
	}
}
