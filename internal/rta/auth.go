package rta

import (
	"fmt"

	ocmconfig "github.com/openshift-online/ocm-cli/pkg/config"
	"github.com/spf13/cobra"
)

// resolveToken returns a bearer token using the precedence:
// --token flag > OCM config file access token.
func (g *commandGroup) resolveToken(cmd *cobra.Command) (string, error) {
	if t := fromFlag(cmd, "token"); t != "" {
		return t, nil
	}

	cfg, err := ocmconfig.Load()
	if err != nil {
		return "", fmt.Errorf("load OCM configuration: %w", err)
	}
	if cfg == nil {
		return "", fmt.Errorf("not authenticated: use --token or log in with ocm")
	}
	if cfg.AccessToken != "" {
		return cfg.AccessToken, nil
	}
	return "", fmt.Errorf("not authenticated: OCM config has no access token; log in with ocm")
}
