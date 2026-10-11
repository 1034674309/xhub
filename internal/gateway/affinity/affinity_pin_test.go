package affinity

import (
	"testing"
	"time"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

func TestOfficialTaskPinLastsSevenDays(t *testing.T) {
	if officialPinTTL != 7*24*time.Hour {
		t.Fatalf("official pin ttl %s", officialPinTTL)
	}
	s := &State{}
	s.PinOfficial("cgt-1", "base|model")
	if got := s.OfficialDeployment("cgt-1"); got != "base|model" {
		t.Fatalf("pin %q", got)
	}
}

func TestOfficialContextSurvivesHostReplacement(t *testing.T) {
	s := &State{}
	facts := providercontract.TaskContext{StartedAt: time.Now().UTC(), Model: "bytedance/model", Resolution: "1080p", HasVideo: true, Known: true}
	s.PinOfficialContext("scope", facts)
	restored := &State{affinity: s.affinity}
	if got := restored.OfficialContext("scope"); got != facts {
		t.Fatalf("context lost: %+v", got)
	}
	pin := s.affinity["official_context:v1:scope"]
	if time.Until(pin.until) < officialPinTTL-time.Minute {
		t.Fatal("wrong context TTL")
	}
	if restored.OfficialContext("other").Known {
		t.Fatal("cross-task context leak")
	}
}
