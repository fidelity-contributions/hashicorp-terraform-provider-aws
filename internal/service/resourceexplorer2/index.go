// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resourceexplorer2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resourceexplorer2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/resourceexplorer2/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_resourceexplorer2_index", name="Index")
// @Tags(identifierAttribute="arn")
// @ArnIdentity(identityDuplicateAttributes="id")
// @Testing(serialize=true)
// @Testing(generator=false)
func newIndexResource(context.Context) (resource.ResourceWithConfigure, error) {
	r := &indexResource{}

	r.SetDefaultCreateTimeout(2 * time.Hour)
	r.SetDefaultUpdateTimeout(2 * time.Hour)
	r.SetDefaultDeleteTimeout(10 * time.Minute)

	return r, nil
}

type indexResource struct {
	framework.ResourceWithModel[indexResourceModel]
	framework.WithImportByIdentity
	framework.WithTimeouts
}

func (r *indexResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrARN:     framework.ARNAttributeComputedOnly(),
			names.AttrID:      framework.IDAttribute(),
			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
			names.AttrType: schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.IndexType](),
				Required:   true,
			},
		},
		Blocks: map[string]schema.Block{
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *indexResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data indexResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResourceExplorer2Client(ctx)

	input := &resourceexplorer2.CreateIndexInput{
		ClientToken: aws.String(sdkid.UniqueId()),
		Tags:        getTagsIn(ctx),
	}

	output, err := conn.CreateIndex(ctx, input)

	if err != nil {
		response.Diagnostics.AddError("creating Resource Explorer Index", err.Error())

		return
	}

	arn := aws.ToString(output.Arn)
	data.ID = types.StringValue(arn)

	createTimeout := r.CreateTimeout(ctx, data.Timeouts)
	if _, err := waitIndexCreated(ctx, conn, createTimeout); err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("waiting for Resource Explorer Index (%s) create", data.ID.ValueString()), err.Error())

		return
	}

	if data.Type.ValueEnum() == awstypes.IndexTypeAggregator {
		input := &resourceexplorer2.UpdateIndexTypeInput{
			Arn:  fwflex.StringFromFramework(ctx, data.ID),
			Type: awstypes.IndexTypeAggregator,
		}

		_, err := conn.UpdateIndexType(ctx, input)

		if err != nil {
			response.State.SetAttribute(ctx, path.Root(names.AttrID), data.ID) // Set 'id' so as to taint the resource.
			response.Diagnostics.AddError(fmt.Sprintf("updating Resource Explorer Index (%s)", data.ID.ValueString()), err.Error())
			return
		}

		if _, err := waitIndexUpdated(ctx, conn, createTimeout); err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("waiting for Resource Explorer Index (%s) update", data.ID.ValueString()), err.Error())

			return
		}
	}

	// Set values for unknowns.
	data.ARN = types.StringValue(arn)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *indexResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data indexResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResourceExplorer2Client(ctx)

	output, err := findIndex(ctx, conn)

	if tfresource.NotFound(err) {
		response.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		response.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading Resource Explorer Index (%s)", data.ID.ValueString()), err.Error())

		return
	}

	response.Diagnostics.Append(fwflex.Flatten(ctx, output, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	setTagsOut(ctx, output.Tags)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *indexResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var old, new indexResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &old)...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(request.Plan.Get(ctx, &new)...)
	if response.Diagnostics.HasError() {
		return
	}

	if !new.Type.Equal(old.Type) {
		conn := r.Meta().ResourceExplorer2Client(ctx)

		input := &resourceexplorer2.UpdateIndexTypeInput{
			Arn:  fwflex.StringFromFramework(ctx, new.ARN),
			Type: new.Type.ValueEnum(),
		}

		_, err := conn.UpdateIndexType(ctx, input)

		if err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("updating Resource Explorer Index (%s)", new.ID.ValueString()), err.Error())

			return
		}

		if _, err := waitIndexUpdated(ctx, conn, r.UpdateTimeout(ctx, new.Timeouts)); err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("waiting for Resource Explorer Index (%s) update", new.ID.ValueString()), err.Error())

			return
		}
	}

	response.Diagnostics.Append(response.State.Set(ctx, &new)...)
}

func (r *indexResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data indexResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResourceExplorer2Client(ctx)

	tflog.Debug(ctx, "deleting Resource Explorer Index", map[string]any{
		names.AttrID: data.ID.ValueString(),
	})
	_, err := conn.DeleteIndex(ctx, &resourceexplorer2.DeleteIndexInput{
		Arn: fwflex.StringFromFramework(ctx, data.ARN),
	})

	if errs.IsAErrorMessageContains[*awstypes.ValidationException](err, "The index is DELETED") {
		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("deleting Resource Explorer Index (%s)", data.ID.ValueString()), err.Error())

		return
	}

	if _, err := waitIndexDeleted(ctx, conn, r.DeleteTimeout(ctx, data.Timeouts)); err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("waiting for Resource Explorer Index (%s) delete", data.ID.ValueString()), err.Error())

		return
	}
}

// See https://docs.aws.amazon.com/resource-explorer/latest/apireference/API_Index.html.
type indexResourceModel struct {
	framework.WithRegionModel
	ARN      types.String                           `tfsdk:"arn"`
	ID       types.String                           `tfsdk:"id"`
	Tags     tftags.Map                             `tfsdk:"tags"`
	TagsAll  tftags.Map                             `tfsdk:"tags_all"`
	Timeouts timeouts.Value                         `tfsdk:"timeouts"`
	Type     fwtypes.StringEnum[awstypes.IndexType] `tfsdk:"type"`
}

func findIndex(ctx context.Context, conn *resourceexplorer2.Client) (*resourceexplorer2.GetIndexOutput, error) {
	input := &resourceexplorer2.GetIndexInput{}

	output, err := conn.GetIndex(ctx, input)

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return nil, &retry.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	if state := output.State; state == awstypes.IndexStateDeleted {
		return nil, &retry.NotFoundError{
			Message:     string(state),
			LastRequest: input,
		}
	}

	return output, nil
}

func statusIndex(ctx context.Context, conn *resourceexplorer2.Client) retry.StateRefreshFunc {
	return func() (any, string, error) {
		output, err := findIndex(ctx, conn)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, string(output.State), nil
	}
}

func waitIndexCreated(ctx context.Context, conn *resourceexplorer2.Client, timeout time.Duration) (*resourceexplorer2.GetIndexOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(awstypes.IndexStateCreating),
		Target:  enum.Slice(awstypes.IndexStateActive),
		Refresh: statusIndex(ctx, conn),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*resourceexplorer2.GetIndexOutput); ok {
		return output, err
	}

	return nil, err
}

func waitIndexUpdated(ctx context.Context, conn *resourceexplorer2.Client, timeout time.Duration) (*resourceexplorer2.GetIndexOutput, error) { //nolint:unparam
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(awstypes.IndexStateUpdating),
		Target:  enum.Slice(awstypes.IndexStateActive),
		Refresh: statusIndex(ctx, conn),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*resourceexplorer2.GetIndexOutput); ok {
		return output, err
	}

	return nil, err
}

func waitIndexDeleted(ctx context.Context, conn *resourceexplorer2.Client, timeout time.Duration) (*resourceexplorer2.GetIndexOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(awstypes.IndexStateDeleting),
		Target:  []string{},
		Refresh: statusIndex(ctx, conn),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*resourceexplorer2.GetIndexOutput); ok {
		return output, err
	}

	return nil, err
}
