// Package factory assembles the ReBAC Provider and Authorizer.
package factory

import (
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/rebac/openfga"
)

// NewAuthorizer creates the ReBAC Authorizer backed by an in-process OpenFGA server.
func NewAuthorizer() (rebac.Authorizer, error) {
	provider, err := openfga.NewDefaultProvider()
	if err != nil {
		return nil, err
	}
	authorizer, err := rebac.NewAuthorizer(provider)
	if err != nil {
		return nil, err
	}

	return authorizer, nil
}
