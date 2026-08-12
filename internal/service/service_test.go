package service

import (
	"testing"

	"github.com/lizhicheng00/relay-controller/internal/core"
)

func TestValidateUpdateTunnelRejectsBlankName(t *testing.T) {
	name := "  "
	if err := validateUpdateTunnel(core.UpdateTunnelRequest{Name: &name}); err == nil {
		t.Fatal("expected blank name to fail")
	}
}

func TestValidateUpdateTunnelRejectsLongDescription(t *testing.T) {
	description := string(make([]byte, 513))
	if err := validateUpdateTunnel(core.UpdateTunnelRequest{Description: &description}); err == nil {
		t.Fatal("expected long description to fail")
	}
}
