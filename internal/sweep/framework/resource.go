// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package framework

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

type attribute struct {
	path  string
	value any
}

func NewAttribute(path string, value any) attribute {
	return attribute{
		path:  path,
		value: value,
	}
}

type sweepResource struct {
	factory    func(context.Context) (fwresource.ResourceWithConfigure, error)
	meta       *conns.AWSClient
	attributes []attribute
}

func NewSweepResource(factory func(context.Context) (fwresource.ResourceWithConfigure, error), meta *conns.AWSClient, attributes ...attribute) *sweepResource {
	return &sweepResource{
		factory:    factory,
		meta:       meta,
		attributes: attributes,
	}
}

func (sr *sweepResource) Delete(ctx context.Context, optFns ...tfresource.OptionsFunc) error {
	resource, err := sr.factory(ctx)
	if err != nil {
		return err
	}

	var configureResp fwresource.ConfigureResponse
	resource.Configure(ctx, fwresource.ConfigureRequest{ProviderData: sr.meta}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		return fwdiag.DiagnosticsError(configureResp.Diagnostics)
	}

	var schemaResp fwresource.SchemaResponse
	resource.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		return fwdiag.DiagnosticsError(schemaResp.Diagnostics)
	}

	state := tfsdk.State{
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		Schema: schemaResp.Schema,
	}
	for _, attr := range sr.attributes {
		d := state.SetAttribute(ctx, path.Root(attr.path), attr.value)
		if d.HasError() {
			return fwdiag.DiagnosticsError(d)
		}
		ctx = tflog.SetField(ctx, attr.path, attr.value)
	}

	tflog.Info(ctx, "Sweeping resource")

	err = deleteResource(ctx, state, resource)

	if errs.Contains(err, "Value Conversion Error") {
		// Hack for per-resource Region override.
		// Inject a top-level region attribute into the schema and retry.
		schema := state.Schema.(rschema.Schema)
		schema.Attributes[names.AttrRegion] = rschema.StringAttribute{
			Optional: true,
			Computed: true,
		}
		state := tfsdk.State{
			Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
			Schema: schema,
		}
		for _, attr := range sr.attributes {
			d := state.SetAttribute(ctx, path.Root(attr.path), attr.value)
			if d.HasError() {
				return fwdiag.DiagnosticsError(d)
			}
		}

		err = deleteResource(ctx, state, resource)
	}

	return err
}

func deleteResource(ctx context.Context, state tfsdk.State, resource fwresource.Resource) error {
	var response fwresource.DeleteResponse
	resource.Delete(ctx, fwresource.DeleteRequest{State: state}, &response)

	return fwdiag.DiagnosticsError(response.Diagnostics)
}
