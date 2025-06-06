// Code generated by internal/generate/servicepackage/main.go; DO NOT EDIT.

package amp

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/types"
	"github.com/hashicorp/terraform-provider-aws/names"
)

type servicePackage struct{}

func (p *servicePackage) FrameworkDataSources(ctx context.Context) []*types.ServicePackageFrameworkDataSource {
	return []*types.ServicePackageFrameworkDataSource{
		{
			Factory:  newDefaultScraperConfigurationDataSource,
			TypeName: "aws_prometheus_default_scraper_configuration",
			Name:     "Default Scraper Configuration",
		},
	}
}

func (p *servicePackage) FrameworkResources(ctx context.Context) []*types.ServicePackageFrameworkResource {
	return []*types.ServicePackageFrameworkResource{
		{
			Factory:  newScraperResource,
			TypeName: "aws_prometheus_scraper",
			Name:     "Scraper",
			Tags: &types.ServicePackageResourceTags{
				IdentifierAttribute: names.AttrARN,
			},
		},
		{
			Factory:  newWorkspaceConfigurationResource,
			TypeName: "aws_prometheus_workspace_configuration",
			Name:     "WorkspaceConfiguration",
		},
	}
}

func (p *servicePackage) SDKDataSources(ctx context.Context) []*types.ServicePackageSDKDataSource {
	return []*types.ServicePackageSDKDataSource{
		{
			Factory:  dataSourceWorkspace,
			TypeName: "aws_prometheus_workspace",
			Name:     "Workspace",
			Tags:     &types.ServicePackageResourceTags{},
		},
		{
			Factory:  dataSourceWorkspaces,
			TypeName: "aws_prometheus_workspaces",
			Name:     "Workspaces",
		},
	}
}

func (p *servicePackage) SDKResources(ctx context.Context) []*types.ServicePackageSDKResource {
	return []*types.ServicePackageSDKResource{
		{
			Factory:  resourceAlertManagerDefinition,
			TypeName: "aws_prometheus_alert_manager_definition",
			Name:     "Alert Manager Definition",
		},
		{
			Factory:  resourceRuleGroupNamespace,
			TypeName: "aws_prometheus_rule_group_namespace",
			Name:     "Rule Group Namespace",
			Tags: &types.ServicePackageResourceTags{
				IdentifierAttribute: names.AttrARN,
			},
		},
		{
			Factory:  resourceWorkspace,
			TypeName: "aws_prometheus_workspace",
			Name:     "Workspace",
			Tags: &types.ServicePackageResourceTags{
				IdentifierAttribute: names.AttrARN,
			},
		},
	}
}

func (p *servicePackage) ServicePackageName() string {
	return names.AMP
}

// NewClient returns a new AWS SDK for Go v2 client for this service package's AWS API.
func (p *servicePackage) NewClient(ctx context.Context, config map[string]any) (*amp.Client, error) {
	cfg := *(config["aws_sdkv2_config"].(*aws.Config))
	optFns := []func(*amp.Options){
		amp.WithEndpointResolverV2(newEndpointResolverV2()),
		withBaseEndpoint(config[names.AttrEndpoint].(string)),
		withExtraOptions(ctx, p, config),
	}

	return amp.NewFromConfig(cfg, optFns...), nil
}

// withExtraOptions returns a functional option that allows this service package to specify extra API client options.
// This option is always called after any generated options.
func withExtraOptions(ctx context.Context, sp conns.ServicePackage, config map[string]any) func(*amp.Options) {
	if v, ok := sp.(interface {
		withExtraOptions(context.Context, map[string]any) []func(*amp.Options)
	}); ok {
		optFns := v.withExtraOptions(ctx, config)

		return func(o *amp.Options) {
			for _, optFn := range optFns {
				optFn(o)
			}
		}
	}

	return func(*amp.Options) {}
}

func ServicePackage(ctx context.Context) conns.ServicePackage {
	return &servicePackage{}
}
