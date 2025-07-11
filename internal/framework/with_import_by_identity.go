// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package framework

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/provider/framework/importer"
	inttypes "github.com/hashicorp/terraform-provider-aws/internal/types"
)

// TODO: Needs a better name
type ImportByIdentityer interface {
	SetIdentitySpec(identity inttypes.Identity, importSpec inttypes.FrameworkImport)
}

var _ ImportByIdentityer = &WithImportByIdentity{}

type WithImportByIdentity struct {
	identity   inttypes.Identity
	importSpec inttypes.FrameworkImport
}

func (w *WithImportByIdentity) SetIdentitySpec(identity inttypes.Identity, importSpec inttypes.FrameworkImport) {
	w.identity = identity
	w.importSpec = importSpec
}

func (w WithImportByIdentity) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	client := importer.Client(ctx)
	if client == nil {
		response.Diagnostics.AddError(
			"Unexpected Error",
			"An unexpected error occurred while importing a resource. "+
				"This is always an error in the provider. "+
				"Please report the following to the provider developer:\n\n"+
				"Importer context was nil.",
		)
		return
	}

	if w.identity.IsARN {
		importer.ARN(ctx, client, request, &w.identity, &w.importSpec, response)
	} else if w.identity.IsSingleton {
		importer.Singleton(ctx, client, request, &w.identity, &w.importSpec, response)
	} else if w.identity.IsSingleParameter {
		importer.SingleParameterized(ctx, client, request, &w.identity, &w.importSpec, response)
	} else {
		importer.MultipleParameterized(ctx, client, request, &w.identity, &w.importSpec, response)
	}
}

func (w WithImportByIdentity) IdentitySpec() inttypes.Identity {
	return w.identity
}

func (w WithImportByIdentity) ImportSpec() inttypes.FrameworkImport {
	return w.importSpec
}
