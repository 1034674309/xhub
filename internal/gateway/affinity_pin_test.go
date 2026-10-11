package gateway

import (
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/gateway/affinity"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

func TestOfficialContextSurvivesRedisHostReplacement(t *testing.T) {
	c := settlementRedis(t)
	s := &Server{State: *affinity.New(c)}
	facts := providercontract.TaskContext{StartedAt: time.Now().UTC(), Model: "bytedance/model", Resolution: "480p", Known: true}
	s.PinOfficialContext("scope", facts)
	restored := &Server{State: *affinity.New(c)}
	if got := restored.OfficialContext("scope"); got != facts {
		t.Fatalf("Redis lost billing facts: %+v", got)
	}
}
