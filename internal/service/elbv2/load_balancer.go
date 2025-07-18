// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package elbv2

import ( // nosemgrep:ci.semgrep.aws.multiple-service-imports
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tfec2 "github.com/hashicorp/terraform-provider-aws/internal/service/ec2"
	tfslices "github.com/hashicorp/terraform-provider-aws/internal/slices"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_alb", name="Load Balancer")
// @SDKResource("aws_lb", name="Load Balancer")
// @Tags(identifierAttribute="arn")
// @ArnIdentity
// @V60SDKv2Fix
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types;awstypes;awstypes.LoadBalancer")
func resourceLoadBalancer() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceLoadBalancerCreate,
		ReadWithoutTimeout:   resourceLoadBalancerRead,
		UpdateWithoutTimeout: resourceLoadBalancerUpdate,
		DeleteWithoutTimeout: resourceLoadBalancerDelete,

		CustomizeDiff: customdiff.Sequence(
			customizeDiffLoadBalancerALB,
			customizeDiffLoadBalancerNLB,
			customizeDiffLoadBalancerGWLB,
		),

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"access_logs": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrBucket: {
							Type:     schema.TypeString,
							Required: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return !d.Get("access_logs.0.enabled").(bool)
							},
						},
						names.AttrEnabled: {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						names.AttrPrefix: {
							Type:     schema.TypeString,
							Optional: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return !d.Get("access_logs.0.enabled").(bool)
							},
						},
					},
				},
			},
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"arn_suffix": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"client_keep_alive": {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          3600,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"connection_logs": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrBucket: {
							Type:     schema.TypeString,
							Required: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return !d.Get("connection_logs.0.enabled").(bool)
							},
						},
						names.AttrEnabled: {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						names.AttrPrefix: {
							Type:     schema.TypeString,
							Optional: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return !d.Get("connection_logs.0.enabled").(bool)
							},
						},
					},
				},
			},
			"customer_owned_ipv4_pool": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"desync_mitigation_mode": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          httpDesyncMitigationModeDefensive,
				ValidateFunc:     validation.StringInSlice(httpDesyncMitigationMode_Values(), false),
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			names.AttrDNSName: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"dns_record_client_routing_policy": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          dnsRecordClientRoutingPolicyAnyAvailabilityZone,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumNetwork),
				ValidateFunc:     validation.StringInSlice(dnsRecordClientRoutingPolicy_Values(), false),
			},
			"drop_invalid_header_fields": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_cross_zone_load_balancing": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBType(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_deletion_protection": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"enable_http2": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          true,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_tls_version_and_cipher_suite_headers": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_waf_fail_open": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_xff_client_port": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"enable_zonal_shift": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication, awstypes.LoadBalancerTypeEnumNetwork),
			},
			"enforce_security_group_inbound_rules_on_private_link_traffic": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ValidateDiagFunc: enum.Validate[awstypes.EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum](),
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumNetwork),
			},
			"idle_timeout": {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          60,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			"internal": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			names.AttrIPAddressType: {
				Type:             schema.TypeString,
				Computed:         true,
				Optional:         true,
				ValidateDiagFunc: enum.Validate[awstypes.IpAddressType](),
			},
			"ipam_pools": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ipv4_ipam_pool_id": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"load_balancer_type": {
				Type:             schema.TypeString,
				ForceNew:         true,
				Optional:         true,
				Default:          awstypes.LoadBalancerTypeEnumApplication,
				ValidateDiagFunc: enum.Validate[awstypes.LoadBalancerTypeEnum](),
			},
			"minimum_load_balancer_capacity": {
				Type:             schema.TypeList,
				Optional:         true,
				MaxItems:         1,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication, awstypes.LoadBalancerTypeEnumNetwork),
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"capacity_units": {
							Type:     schema.TypeInt,
							Required: true,
						},
					},
				},
			},
			names.AttrName: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrNamePrefix},
				ValidateFunc:  validName,
			},
			names.AttrNamePrefix: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrName},
				ValidateFunc:  validNamePrefix,
			},
			"preserve_host_header": {
				Type:             schema.TypeBool,
				Optional:         true,
				Default:          false,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
			},
			names.AttrSecurityGroups: {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"subnet_mapping": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allocation_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"ipv6_address": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.IsIPv6Address,
						},
						"outpost_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"private_ipv4_address": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.IsIPv4Address,
						},
						names.AttrSubnetID: {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				ExactlyOneOf: []string{"subnet_mapping", names.AttrSubnets},
			},
			names.AttrSubnets: {
				Type:         schema.TypeSet,
				Optional:     true,
				Computed:     true,
				Elem:         &schema.Schema{Type: schema.TypeString},
				ExactlyOneOf: []string{"subnet_mapping", names.AttrSubnets},
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
			names.AttrVPCID: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"xff_header_processing_mode": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          httpXFFHeaderProcessingModeAppend,
				DiffSuppressFunc: suppressIfLBTypeNot(awstypes.LoadBalancerTypeEnumApplication),
				ValidateFunc:     validation.StringInSlice(httpXFFHeaderProcessingMode_Values(), false),
			},
			"zone_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func suppressIfLBType(types ...awstypes.LoadBalancerTypeEnum) schema.SchemaDiffSuppressFunc {
	return func(k string, old string, new string, d *schema.ResourceData) bool {
		return slices.Contains(types, awstypes.LoadBalancerTypeEnum(d.Get("load_balancer_type").(string)))
	}
}

func suppressIfLBTypeNot(types ...awstypes.LoadBalancerTypeEnum) schema.SchemaDiffSuppressFunc {
	return func(k string, old string, new string, d *schema.ResourceData) bool {
		return !slices.Contains(types, awstypes.LoadBalancerTypeEnum(d.Get("load_balancer_type").(string)))
	}
}

func resourceLoadBalancerCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ELBV2Client(ctx)
	partition := meta.(*conns.AWSClient).Partition(ctx)

	name := create.NewNameGenerator(
		create.WithConfiguredName(d.Get(names.AttrName).(string)),
		create.WithConfiguredPrefix(d.Get(names.AttrNamePrefix).(string)),
		create.WithDefaultPrefix("tf-lb-"),
	).Generate()
	exist, err := findLoadBalancer(ctx, conn, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		Names: []string{name},
	})

	if err != nil && !tfresource.NotFound(err) {
		return sdkdiag.AppendErrorf(diags, "reading ELBv2 Load Balancer (%s): %s", name, err)
	}

	if exist != nil {
		return sdkdiag.AppendErrorf(diags, "ELBv2 Load Balancer (%s) already exists", name)
	}

	d.Set(names.AttrName, name)

	lbType := awstypes.LoadBalancerTypeEnum(d.Get("load_balancer_type").(string))
	input := &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name: aws.String(name),
		Tags: getTagsIn(ctx),
		Type: lbType,
	}

	if v, ok := d.GetOk("customer_owned_ipv4_pool"); ok {
		input.CustomerOwnedIpv4Pool = aws.String(v.(string))
	}

	if _, ok := d.GetOk("internal"); ok {
		input.Scheme = awstypes.LoadBalancerSchemeEnumInternal
	}

	if v, ok := d.GetOk(names.AttrIPAddressType); ok {
		input.IpAddressType = awstypes.IpAddressType(v.(string))
	}

	if v, ok := d.GetOk("ipam_pools"); ok {
		input.IpamPools = expandIPAMPools(v.([]any))
	}

	if v, ok := d.GetOk(names.AttrSecurityGroups); ok {
		input.SecurityGroups = flex.ExpandStringValueSet(v.(*schema.Set))
	}

	if v, ok := d.GetOk("subnet_mapping"); ok && v.(*schema.Set).Len() > 0 {
		input.SubnetMappings = expandSubnetMappings(v.(*schema.Set).List())
	}

	if v, ok := d.GetOk(names.AttrSubnets); ok {
		input.Subnets = flex.ExpandStringValueSet(v.(*schema.Set))
	}

	output, err := conn.CreateLoadBalancer(ctx, input)

	// Some partitions (e.g. ISO) may not support tag-on-create.
	if input.Tags != nil && errs.IsUnsupportedOperationInPartitionError(partition, err) {
		input.Tags = nil

		output, err = conn.CreateLoadBalancer(ctx, input)
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating ELBv2 %s Load Balancer (%s): %s", lbType, name, err)
	}

	d.SetId(aws.ToString(output.LoadBalancers[0].LoadBalancerArn))

	if _, err := waitLoadBalancerActive(ctx, conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for ELBv2 Load Balancer (%s) create: %s", d.Id(), err)
	}

	// For partitions not supporting tag-on-create, attempt tag after create.
	if tags := getTagsIn(ctx); input.Tags == nil && len(tags) > 0 {
		err := createTags(ctx, conn, d.Id(), tags)

		// If default tags only, continue. Otherwise, error.
		if v, ok := d.GetOk(names.AttrTags); (!ok || len(v.(map[string]any)) == 0) && errs.IsUnsupportedOperationInPartitionError(partition, err) {
			return append(diags, resourceLoadBalancerUpdate(ctx, d, meta)...)
		}

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ELBv2 Load Balancer (%s) tags: %s", d.Id(), err)
		}
	}

	var attributes []awstypes.LoadBalancerAttribute
	var minCapacity *awstypes.MinimumLoadBalancerCapacity

	if lbType == awstypes.LoadBalancerTypeEnumApplication || lbType == awstypes.LoadBalancerTypeEnumNetwork {
		if v, ok := d.GetOk("access_logs"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			attributes = append(attributes, expandLoadBalancerAccessLogsAttributes(v.([]any)[0].(map[string]any), false)...)
		} else {
			attributes = append(attributes, awstypes.LoadBalancerAttribute{
				Key:   aws.String(loadBalancerAttributeAccessLogsS3Enabled),
				Value: flex.BoolValueToString(false),
			})
		}
		if v, ok := d.GetOk("minimum_load_balancer_capacity"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			minCapacity = expandMinimumLoadBalancerCapacity(v.([]any))
		}
	}

	if lbType == awstypes.LoadBalancerTypeEnumApplication {
		if v, ok := d.GetOk("connection_logs"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			attributes = append(attributes, expandLoadBalancerConnectionLogsAttributes(v.([]any)[0].(map[string]any), false)...)
		} else {
			attributes = append(attributes, awstypes.LoadBalancerAttribute{
				Key:   aws.String(loadBalancerAttributeConnectionLogsS3Enabled),
				Value: flex.BoolValueToString(false),
			})
		}
	}

	attributes = append(attributes, loadBalancerAttributes.expand(d, lbType, false)...)

	if minCapacity != nil {
		if err := modifyCapacityReservation(ctx, conn, d.Id(), minCapacity); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}

		if _, err := waitCapacityReservationProvisioned(ctx, conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
			return sdkdiag.AppendErrorf(diags, "waiting for ELBv2 Load Balancer (%s) capacity reservation provision: %s", d.Id(), err)
		}
	}

	wait := false
	if len(attributes) > 0 {
		if err := modifyLoadBalancerAttributes(ctx, conn, d.Id(), attributes); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}

		wait = true
	}

	if v, ok := d.GetOk("enforce_security_group_inbound_rules_on_private_link_traffic"); ok && lbType == awstypes.LoadBalancerTypeEnumNetwork {
		input := &elasticloadbalancingv2.SetSecurityGroupsInput{
			EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic: awstypes.EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum(v.(string)),
			LoadBalancerArn: aws.String(d.Id()),
		}

		if v, ok := d.GetOk(names.AttrSecurityGroups); ok {
			input.SecurityGroups = flex.ExpandStringValueSet(v.(*schema.Set))
		}

		_, err := conn.SetSecurityGroups(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ELBv2 Load Balancer (%s) security groups: %s", d.Id(), err)
		}

		wait = true
	}

	if wait {
		if _, err := waitLoadBalancerActive(ctx, conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
			return sdkdiag.AppendErrorf(diags, "waiting for ELBv2 Load Balancer (%s) create: %s", d.Id(), err)
		}
	}

	return append(diags, resourceLoadBalancerRead(ctx, d, meta)...)
}

func resourceLoadBalancerRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ELBV2Client(ctx)

	lb, err := findLoadBalancerByARN(ctx, conn, d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] ELBv2 Load Balancer %s not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading ELBv2 Load Balancer (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrARN, lb.LoadBalancerArn)
	d.Set("arn_suffix", suffixFromARN(lb.LoadBalancerArn))
	d.Set("customer_owned_ipv4_pool", lb.CustomerOwnedIpv4Pool)
	d.Set(names.AttrDNSName, lb.DNSName)
	d.Set("enforce_security_group_inbound_rules_on_private_link_traffic", lb.EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic)
	d.Set("internal", lb.Scheme == awstypes.LoadBalancerSchemeEnumInternal)
	d.Set(names.AttrIPAddressType, lb.IpAddressType)
	if err := d.Set("ipam_pools", flattenIPAMPools(lb.IpamPools)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting ipam_pools: %s", err)
	}
	d.Set("load_balancer_type", lb.Type)
	d.Set(names.AttrName, lb.LoadBalancerName)
	d.Set(names.AttrNamePrefix, create.NamePrefixFromName(aws.ToString(lb.LoadBalancerName)))
	d.Set(names.AttrSecurityGroups, lb.SecurityGroups)
	if err := d.Set("subnet_mapping", flattenSubnetMappingsFromAvailabilityZones(lb.AvailabilityZones)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting subnet_mapping: %s", err)
	}
	if err := d.Set(names.AttrSubnets, flattenSubnetsFromAvailabilityZones(lb.AvailabilityZones)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting subnets: %s", err)
	}
	d.Set(names.AttrVPCID, lb.VpcId)
	d.Set("zone_id", lb.CanonicalHostedZoneId)

	attributes, err := findLoadBalancerAttributesByARN(ctx, conn, d.Id())

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading ELBv2 Load Balancer (%s) attributes: %s", d.Id(), err)
	}

	if err := d.Set("access_logs", []any{flattenLoadBalancerAccessLogsAttributes(attributes)}); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting access_logs: %s", err)
	}

	if lb.Type == awstypes.LoadBalancerTypeEnumApplication {
		if err := d.Set("connection_logs", []any{flattenLoadBalancerConnectionLogsAttributes(attributes)}); err != nil {
			return sdkdiag.AppendErrorf(diags, "setting connection_logs: %s", err)
		}
	}

	loadBalancerAttributes.flatten(d, attributes)

	if lb.Type == awstypes.LoadBalancerTypeEnumApplication || lb.Type == awstypes.LoadBalancerTypeEnumNetwork {
		capacity, err := findCapacityReservationByARN(ctx, conn, d.Id())

		switch {
		case tfawserr.ErrCodeEquals(err, errCodeAccessDenied, errCodeInvalidAction):
			d.Set("minimum_load_balancer_capacity", nil)
		case err != nil:
			return sdkdiag.AppendErrorf(diags, "reading ELBv2 Load Balancer (%s) capacity reservation: %s", d.Id(), err)
		default:
			if err := d.Set("minimum_load_balancer_capacity", flattenMinimumLoadBalancerCapacity(capacity.MinimumLoadBalancerCapacity)); err != nil {
				return sdkdiag.AppendErrorf(diags, "setting minimum_load_balancer_capacity: %s", err)
			}
		}
	}

	return diags
}

func resourceLoadBalancerUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ELBV2Client(ctx)

	lbType := awstypes.LoadBalancerTypeEnum(d.Get("load_balancer_type").(string))
	var attributes []awstypes.LoadBalancerAttribute

	if d.HasChange("access_logs") {
		if v, ok := d.GetOk("access_logs"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			attributes = append(attributes, expandLoadBalancerAccessLogsAttributes(v.([]any)[0].(map[string]any), true)...)
		} else {
			attributes = append(attributes, awstypes.LoadBalancerAttribute{
				Key:   aws.String(loadBalancerAttributeAccessLogsS3Enabled),
				Value: flex.BoolValueToString(false),
			})
		}
	}

	if d.HasChange("connection_logs") {
		if v, ok := d.GetOk("connection_logs"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			attributes = append(attributes, expandLoadBalancerConnectionLogsAttributes(v.([]any)[0].(map[string]any), true)...)
		} else {
			attributes = append(attributes, awstypes.LoadBalancerAttribute{
				Key:   aws.String(loadBalancerAttributeConnectionLogsS3Enabled),
				Value: flex.BoolValueToString(false),
			})
		}
	}

	attributes = append(attributes, loadBalancerAttributes.expand(d, lbType, true)...)

	if len(attributes) > 0 {
		if err := modifyLoadBalancerAttributes(ctx, conn, d.Id(), attributes); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	if d.HasChanges("enforce_security_group_inbound_rules_on_private_link_traffic", names.AttrSecurityGroups) {
		input := &elasticloadbalancingv2.SetSecurityGroupsInput{
			LoadBalancerArn: aws.String(d.Id()),
			SecurityGroups:  flex.ExpandStringValueSet(d.Get(names.AttrSecurityGroups).(*schema.Set)),
		}

		if lbType == awstypes.LoadBalancerTypeEnumNetwork {
			if v, ok := d.GetOk("enforce_security_group_inbound_rules_on_private_link_traffic"); ok {
				input.EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic = awstypes.EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum(v.(string))
			}
		}

		_, err := conn.SetSecurityGroups(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ELBv2 Load Balancer (%s) security groups: %s", d.Id(), err)
		}
	}

	if d.HasChanges("subnet_mapping", names.AttrSubnets) {
		input := &elasticloadbalancingv2.SetSubnetsInput{
			LoadBalancerArn: aws.String(d.Id()),
		}

		if d.HasChange("subnet_mapping") {
			if v, ok := d.GetOk("subnet_mapping"); ok && v.(*schema.Set).Len() > 0 {
				input.SubnetMappings = expandSubnetMappings(v.(*schema.Set).List())
			}
		}

		if d.HasChange(names.AttrSubnets) {
			if v, ok := d.GetOk(names.AttrSubnets); ok {
				input.Subnets = flex.ExpandStringValueSet(v.(*schema.Set))
			}
		}

		_, err := conn.SetSubnets(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ELBv2 Load Balancer (%s) subnets: %s", d.Id(), err)
		}
	}

	if d.HasChange(names.AttrIPAddressType) {
		input := &elasticloadbalancingv2.SetIpAddressTypeInput{
			IpAddressType:   awstypes.IpAddressType(d.Get(names.AttrIPAddressType).(string)),
			LoadBalancerArn: aws.String(d.Id()),
		}

		_, err := conn.SetIpAddressType(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ELBv2 Load Balancer (%s) address type: %s", d.Id(), err)
		}
	}

	if d.HasChange("ipam_pools") {
		input := elasticloadbalancingv2.ModifyIpPoolsInput{
			LoadBalancerArn: aws.String(d.Id()),
		}
		if ipamPools := expandIPAMPools(d.Get("ipam_pools").([]any)); ipamPools == nil {
			input.RemoveIpamPools = []awstypes.RemoveIpamPoolEnum{awstypes.RemoveIpamPoolEnumIpv4}
		} else {
			input.IpamPools = ipamPools
		}

		_, err := conn.ModifyIpPools(ctx, &input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "modifying ELBv2 Load Balancer (%s) IPAM pools: %s", d.Id(), err)
		}
	}

	if d.HasChange("minimum_load_balancer_capacity") {
		if err := modifyCapacityReservation(ctx, conn, d.Id(), expandMinimumLoadBalancerCapacity(d.Get("minimum_load_balancer_capacity").([]any))); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}

		if _, err := waitCapacityReservationProvisioned(ctx, conn, d.Id(), d.Timeout(schema.TimeoutUpdate)); err != nil {
			return sdkdiag.AppendErrorf(diags, "waiting for ELBv2 Load Balancer (%s) capacity reservation provision: %s", d.Id(), err)
		}
	}

	if _, err := waitLoadBalancerActive(ctx, conn, d.Id(), d.Timeout(schema.TimeoutUpdate)); err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for ELBv2 Load Balancer (%s) update: %s", d.Id(), err)
	}

	return append(diags, resourceLoadBalancerRead(ctx, d, meta)...)
}

func resourceLoadBalancerDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ELBV2Client(ctx)

	var ipv4IPAMPoolID string
	if v, ok := d.GetOk("ipam_pools"); ok {
		ipamPools := expandIPAMPools(v.([]any))
		ipv4IPAMPoolID = aws.ToString(ipamPools.Ipv4IpamPoolId)
	}

	log.Printf("[INFO] Deleting ELBv2 Load Balancer: %s", d.Id())
	_, err := conn.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(d.Id()),
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting ELBv2 Load Balancer (%s): %s", d.Id(), err)
	}

	ec2conn := meta.(*conns.AWSClient).EC2Client(ctx)

	if err := waitForALBNetworkInterfacesToDetach(ctx, ec2conn, d.Id(), ipv4IPAMPoolID); err != nil {
		log.Printf("[WARN] Failed to wait for ENIs to disappear for ALB (%s): %s", d.Id(), err)
	}

	if err := waitForNLBNetworkInterfacesToDetach(ctx, ec2conn, d.Id()); err != nil {
		log.Printf("[WARN] Failed to wait for ENIs to disappear for NLB (%s): %s", d.Id(), err)
	}

	return diags
}

func modifyLoadBalancerAttributes(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string, attributes []awstypes.LoadBalancerAttribute) error {
	input := elasticloadbalancingv2.ModifyLoadBalancerAttributesInput{
		Attributes:      attributes,
		LoadBalancerArn: aws.String(arn),
	}

	// Not all attributes are supported in all partitions.
	for {
		if len(input.Attributes) == 0 {
			return nil
		}

		_, err := conn.ModifyLoadBalancerAttributes(ctx, &input)

		if err != nil {
			// "Validation error: Load balancer attribute key 'routing.http.desync_mitigation_mode' is not recognized"
			// "InvalidConfigurationRequest: Load balancer attribute key 'dns_record.client_routing_policy' is not supported on load balancers with type 'network'"
			re := regexache.MustCompile(`attribute key ('|")?([^'" ]+)('|")? is not (recognized|supported)`)
			if sm := re.FindStringSubmatch(err.Error()); len(sm) > 1 {
				key := sm[2]
				input.Attributes = slices.DeleteFunc(input.Attributes, func(v awstypes.LoadBalancerAttribute) bool {
					return aws.ToString(v.Key) == key
				})

				continue
			}

			return fmt.Errorf("modifying ELBv2 Load Balancer (%s) attributes: %w", arn, err)
		}

		return nil
	}
}

func modifyCapacityReservation(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string, minCapacity *awstypes.MinimumLoadBalancerCapacity) error {
	resetCapacityReservation := false
	if minCapacity == nil {
		resetCapacityReservation = true
	} else if minCapacity.CapacityUnits == nil {
		resetCapacityReservation = true
	}
	input := elasticloadbalancingv2.ModifyCapacityReservationInput{
		LoadBalancerArn:             aws.String(arn),
		MinimumLoadBalancerCapacity: minCapacity,
		ResetCapacityReservation:    aws.Bool(resetCapacityReservation),
	}

	_, err := conn.ModifyCapacityReservation(ctx, &input)

	if err != nil {
		return fmt.Errorf("modifying ELBv2 Load Balancer (%s) capacity reservation: %w", arn, err)
	}

	return nil
}

type loadBalancerAttributeInfo struct {
	apiAttributeKey            string
	tfType                     schema.ValueType
	loadBalancerTypesSupported []awstypes.LoadBalancerTypeEnum
}

type loadBalancerAttributeMap map[string]loadBalancerAttributeInfo

var loadBalancerAttributes = loadBalancerAttributeMap(map[string]loadBalancerAttributeInfo{
	"client_keep_alive": {
		apiAttributeKey:            loadBalancerAttributeClientKeepAliveSeconds,
		tfType:                     schema.TypeInt,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"desync_mitigation_mode": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPDesyncMitigationMode,
		tfType:                     schema.TypeString,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"dns_record_client_routing_policy": {
		apiAttributeKey:            loadBalancerAttributeDNSRecordClientRoutingPolicy,
		tfType:                     schema.TypeString,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumNetwork},
	},
	"drop_invalid_header_fields": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPDropInvalidHeaderFieldsEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"enable_cross_zone_load_balancing": {
		apiAttributeKey: loadBalancerAttributeLoadBalancingCrossZoneEnabled,
		tfType:          schema.TypeBool,
		// Although this attribute is supported for ALBs, it must always be true.
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumNetwork, awstypes.LoadBalancerTypeEnumGateway},
	},
	"enable_deletion_protection": {
		apiAttributeKey:            loadBalancerAttributeDeletionProtectionEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication, awstypes.LoadBalancerTypeEnumNetwork, awstypes.LoadBalancerTypeEnumGateway},
	},
	"enable_http2": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTP2Enabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"enable_tls_version_and_cipher_suite_headers": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPXAmznTLSVersionAndCipherSuiteEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"enable_waf_fail_open": {
		apiAttributeKey:            loadBalancerAttributeWAFFailOpenEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"enable_xff_client_port": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPXFFClientPortEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"enable_zonal_shift": {
		apiAttributeKey:            loadBalancerAttributeZonalShiftConfigEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication, awstypes.LoadBalancerTypeEnumNetwork},
	},
	"idle_timeout": {
		apiAttributeKey:            loadBalancerAttributeIdleTimeoutTimeoutSeconds,
		tfType:                     schema.TypeInt,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"preserve_host_header": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPPreserveHostHeaderEnabled,
		tfType:                     schema.TypeBool,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
	"xff_header_processing_mode": {
		apiAttributeKey:            loadBalancerAttributeRoutingHTTPXFFHeaderProcessingMode,
		tfType:                     schema.TypeString,
		loadBalancerTypesSupported: []awstypes.LoadBalancerTypeEnum{awstypes.LoadBalancerTypeEnumApplication},
	},
})

func (m loadBalancerAttributeMap) expand(d *schema.ResourceData, lbType awstypes.LoadBalancerTypeEnum, update bool) []awstypes.LoadBalancerAttribute {
	var apiObjects []awstypes.LoadBalancerAttribute

	for tfAttributeName, attributeInfo := range m {
		// Skip if an update and the attribute hasn't changed.
		if update && !d.HasChange(tfAttributeName) {
			continue
		}

		// Not all attributes are supported on all LB types.
		if !slices.Contains(attributeInfo.loadBalancerTypesSupported, lbType) {
			continue
		}

		switch v, t, k := d.Get(tfAttributeName), attributeInfo.tfType, aws.String(attributeInfo.apiAttributeKey); t {
		case schema.TypeBool:
			v := v.(bool)
			apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
				Key:   k,
				Value: flex.BoolValueToString(v),
			})
		case schema.TypeInt:
			v := v.(int)
			apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
				Key:   k,
				Value: flex.IntValueToString(v),
			})
		case schema.TypeString:
			if v := v.(string); v != "" {
				apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
					Key:   k,
					Value: aws.String(v),
				})
			}
		}
	}

	return apiObjects
}

func (m loadBalancerAttributeMap) flatten(d *schema.ResourceData, apiObjects []awstypes.LoadBalancerAttribute) {
	for tfAttributeName, attributeInfo := range m {
		k := attributeInfo.apiAttributeKey
		i := slices.IndexFunc(apiObjects, func(v awstypes.LoadBalancerAttribute) bool {
			return aws.ToString(v.Key) == k
		})

		if i == -1 {
			continue
		}

		switch v, t := apiObjects[i].Value, attributeInfo.tfType; t {
		case schema.TypeBool:
			d.Set(tfAttributeName, flex.StringToBoolValue(v))
		case schema.TypeInt:
			d.Set(tfAttributeName, flex.StringToIntValue(v))
		case schema.TypeString:
			d.Set(tfAttributeName, v)
		}
	}
}

func findLoadBalancer(ctx context.Context, conn *elasticloadbalancingv2.Client, input *elasticloadbalancingv2.DescribeLoadBalancersInput) (*awstypes.LoadBalancer, error) {
	output, err := findLoadBalancers(ctx, conn, input)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findLoadBalancers(ctx context.Context, conn *elasticloadbalancingv2.Client, input *elasticloadbalancingv2.DescribeLoadBalancersInput) ([]awstypes.LoadBalancer, error) {
	var output []awstypes.LoadBalancer

	pages := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if errs.IsA[*awstypes.LoadBalancerNotFoundException](err) {
			return nil, &retry.NotFoundError{
				LastError:   err,
				LastRequest: input,
			}
		}

		if err != nil {
			return nil, err
		}

		output = append(output, page.LoadBalancers...)
	}

	return output, nil
}

func findLoadBalancerByARN(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string) (*awstypes.LoadBalancer, error) {
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{arn},
	}

	output, err := findLoadBalancer(ctx, conn, input)

	if err != nil {
		return nil, err
	}

	// Eventual consistency check.
	if aws.ToString(output.LoadBalancerArn) != arn {
		return nil, &retry.NotFoundError{
			LastRequest: input,
		}
	}

	return output, nil
}

func findLoadBalancerAttributesByARN(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string) ([]awstypes.LoadBalancerAttribute, error) {
	input := elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
		LoadBalancerArn: aws.String(arn),
	}

	output, err := conn.DescribeLoadBalancerAttributes(ctx, &input)

	if errs.IsA[*awstypes.LoadBalancerNotFoundException](err) {
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

	return output.Attributes, nil
}

func findCapacityReservationByARN(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string) (*elasticloadbalancingv2.DescribeCapacityReservationOutput, error) {
	input := elasticloadbalancingv2.DescribeCapacityReservationInput{
		LoadBalancerArn: aws.String(arn),
	}

	output, err := conn.DescribeCapacityReservation(ctx, &input)

	if errs.IsA[*awstypes.LoadBalancerNotFoundException](err) {
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

	return output, nil
}

func statusLoadBalancer(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string) retry.StateRefreshFunc {
	return func() (any, string, error) {
		output, err := findLoadBalancerByARN(ctx, conn, arn)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, string(output.State.Code), nil
	}
}

func statusCapacityReservation(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string) retry.StateRefreshFunc {
	return func() (any, string, error) {
		output, err := findCapacityReservationByARN(ctx, conn, arn)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		overallState := awstypes.CapacityReservationStateEnumProvisioned
		for _, rs := range output.CapacityReservationState {
			if rs.State.Code != awstypes.CapacityReservationStateEnumProvisioned {
				overallState = rs.State.Code
			}
		}

		return output, string(overallState), nil
	}
}

func waitLoadBalancerActive(ctx context.Context, conn *elasticloadbalancingv2.Client, arn string, timeout time.Duration) (*awstypes.LoadBalancer, error) { //nolint:unparam
	stateConf := &retry.StateChangeConf{
		Pending:    enum.Slice(awstypes.LoadBalancerStateEnumProvisioning, awstypes.LoadBalancerStateEnumFailed),
		Target:     enum.Slice(awstypes.LoadBalancerStateEnumActive),
		Refresh:    statusLoadBalancer(ctx, conn, arn),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
		Delay:      30 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*awstypes.LoadBalancer); ok {
		tfresource.SetLastError(err, errors.New(aws.ToString(output.State.Reason)))

		return output, err
	}

	return nil, err
}

func waitForALBNetworkInterfacesToDetach(ctx context.Context, conn *ec2.Client, arn, ipv4IPAMPoolID string) error {
	name, err := loadBalancerNameFromARN(arn)
	if err != nil {
		return err
	}

	const (
		timeout = 35 * time.Minute // IPAM eventual consistency. It can take ~30 min to release allocations.
	)
	_, err = tfresource.RetryUntilEqual(ctx, timeout, 0, func(ctx context.Context) (int, error) {
		networkInterfaces, err := tfec2.FindNetworkInterfacesByAttachmentInstanceOwnerIDAndDescription(ctx, conn, "amazon-elb", "ELB "+name)
		if err != nil {
			return 0, err
		}

		for _, v := range networkInterfaces {
			if v.Attachment == nil {
				continue
			}

			if ipv4IPAMPoolID != "" {
				if _, err := tfresource.RetryUntilNotFound(ctx, timeout, func(ctx context.Context) (any, error) {
					output, err := tfec2.FindIPAMPoolAllocationsByIPAMPoolIDAndResourceID(ctx, conn, aws.ToString(v.Association.AllocationId), ipv4IPAMPoolID)
					if err != nil {
						return nil, err
					}

					if len(output) == 0 {
						return nil, &retry.NotFoundError{}
					}

					return output, nil
				}); err != nil {
					return 0, err
				}
			}
		}

		return len(networkInterfaces), nil
	})

	return err
}

func waitForNLBNetworkInterfacesToDetach(ctx context.Context, conn *ec2.Client, lbArn string) error {
	name, err := loadBalancerNameFromARN(lbArn)
	if err != nil {
		return err
	}

	const (
		timeout = 5 * time.Minute
	)
	_, err = tfresource.RetryUntilEqual(ctx, timeout, 0, func(ctx context.Context) (int, error) {
		networkInterfaces, err := tfec2.FindNetworkInterfacesByAttachmentInstanceOwnerIDAndDescription(ctx, conn, "amazon-aws", "ELB "+name)
		if err != nil {
			return 0, err
		}

		return len(networkInterfaces), nil
	})

	return err
}

func waitCapacityReservationProvisioned(ctx context.Context, conn *elasticloadbalancingv2.Client, lbArn string, timeout time.Duration) (*elasticloadbalancingv2.DescribeCapacityReservationOutput, error) { //nolint:unparam
	stateConf := &retry.StateChangeConf{
		Pending:    enum.Slice(awstypes.CapacityReservationStateEnumPending, awstypes.CapacityReservationStateEnumFailed, awstypes.CapacityReservationStateEnumRebalancing),
		Target:     enum.Slice(awstypes.CapacityReservationStateEnumProvisioned),
		Refresh:    statusCapacityReservation(ctx, conn, lbArn),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
		Delay:      30 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*elasticloadbalancingv2.DescribeCapacityReservationOutput); ok {
		return output, err
	}

	return nil, err
}

func loadBalancerNameFromARN(s string) (string, error) {
	v, err := arn.Parse(s)
	if err != nil {
		return "", err
	}

	matches := regexache.MustCompile("([^/]+/[^/]+/[^/]+)$").FindStringSubmatch(v.Resource)
	if len(matches) != 2 {
		return "", fmt.Errorf("unexpected ELBv2 Load Balancer ARN format: %q", s)
	}

	// e.g. app/example-alb/b26e625cdde161e6
	return matches[1], nil
}

func flattenSubnetsFromAvailabilityZones(apiObjects []awstypes.AvailabilityZone) []string {
	return tfslices.ApplyToAll(apiObjects, func(apiObject awstypes.AvailabilityZone) string {
		return aws.ToString(apiObject.SubnetId)
	})
}

func flattenSubnetMappingsFromAvailabilityZones(apiObjects []awstypes.AvailabilityZone) []map[string]any {
	return tfslices.ApplyToAll(apiObjects, func(apiObject awstypes.AvailabilityZone) map[string]any {
		tfMap := map[string]any{
			"outpost_id":       aws.ToString(apiObject.OutpostId),
			names.AttrSubnetID: aws.ToString(apiObject.SubnetId),
		}
		if apiObjects := apiObject.LoadBalancerAddresses; len(apiObjects) > 0 {
			apiObject := apiObjects[0]
			tfMap["allocation_id"] = aws.ToString(apiObject.AllocationId)
			tfMap["ipv6_address"] = aws.ToString(apiObject.IPv6Address)
			tfMap["private_ipv4_address"] = aws.ToString(apiObject.PrivateIPv4Address)
		}

		return tfMap
	})
}

func suffixFromARN(arn *string) string {
	if arn == nil {
		return ""
	}

	if arnComponents := regexache.MustCompile(`arn:.*:loadbalancer/(.*)`).FindAllStringSubmatch(*arn, -1); len(arnComponents) == 1 {
		if len(arnComponents[0]) == 2 {
			return arnComponents[0][1]
		}
	}

	return ""
}

// Load balancers of type 'network' cannot have their subnets updated,
// cannot have security groups added if none are present, and cannot have
// all security groups removed. If the type is 'network' and any of these
// conditions are met, mark the diff as a ForceNew operation.
func customizeDiffLoadBalancerNLB(_ context.Context, diff *schema.ResourceDiff, v any) error {
	// The current criteria for determining if the operation should be ForceNew:
	// - lb of type "network"
	// - existing resource (id is not "")
	// - there are subnet removals
	//   OR security groups are being added where none currently exist
	//   OR all security groups are being removed
	//
	// Any other combination should be treated as normal. At this time, subnet
	// handling is the only known difference between Network Load Balancers and
	// Application Load Balancers, so the logic below is simple individual checks.
	// If other differences arise we'll want to refactor to check other
	// conditions in combinations, but for now all we handle is subnets
	if lbType := awstypes.LoadBalancerTypeEnum(diff.Get("load_balancer_type").(string)); lbType != awstypes.LoadBalancerTypeEnumNetwork {
		return nil
	}

	if diff.Id() == "" {
		return nil
	}

	config := diff.GetRawConfig()

	// Subnet diffs.
	// Check for changes here -- SetNewComputed will modify HasChange.
	hasSubnetMappingChanges, hasSubnetsChanges := diff.HasChange("subnet_mapping"), diff.HasChange(names.AttrSubnets)
	if hasSubnetMappingChanges {
		if v := config.GetAttr("subnet_mapping"); v.IsWhollyKnown() {
			o, n := diff.GetChange("subnet_mapping")
			os, ns := o.(*schema.Set), n.(*schema.Set)

			deltaN := ns.Len() - os.Len()
			switch {
			case deltaN == 0:
				// No change in number of subnet mappings, but one of the mappings did change.
				fallthrough
			case deltaN < 0:
				// Subnet mappings removed.
				if err := diff.ForceNew("subnet_mapping"); err != nil {
					return err
				}
			case deltaN > 0:
				// Subnet mappings added. Ensure that the previous mappings didn't change.
				if ns.Intersection(os).Len() != os.Len() {
					if err := diff.ForceNew("subnet_mapping"); err != nil {
						return err
					}
				}
			}
		}

		if err := diff.SetNewComputed(names.AttrSubnets); err != nil {
			return err
		}
	}
	if hasSubnetsChanges {
		if v := config.GetAttr(names.AttrSubnets); v.IsWhollyKnown() {
			o, n := diff.GetChange(names.AttrSubnets)
			os, ns := o.(*schema.Set), n.(*schema.Set)

			// In-place increase in number of subnets only.
			if deltaN := ns.Len() - os.Len(); deltaN <= 0 {
				if err := diff.ForceNew(names.AttrSubnets); err != nil {
					return err
				}
			}
		}

		if err := diff.SetNewComputed("subnet_mapping"); err != nil {
			return err
		}
	}

	// Get diff for security groups.
	if diff.HasChange(names.AttrSecurityGroups) {
		if v := config.GetAttr(names.AttrSecurityGroups); v.IsWhollyKnown() {
			o, n := diff.GetChange(names.AttrSecurityGroups)
			os, ns := o.(*schema.Set), n.(*schema.Set)

			if (os.Len() == 0 && ns.Len() > 0) || (ns.Len() == 0 && os.Len() > 0) {
				if err := diff.ForceNew(names.AttrSecurityGroups); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func customizeDiffLoadBalancerALB(_ context.Context, diff *schema.ResourceDiff, v any) error {
	if lbType := awstypes.LoadBalancerTypeEnum(diff.Get("load_balancer_type").(string)); lbType != awstypes.LoadBalancerTypeEnumApplication {
		return nil
	}

	if diff.Id() == "" {
		return nil
	}

	config := diff.GetRawConfig()

	// Subnet diffs.
	// Check for changes here -- SetNewComputed will modify HasChange.
	hasSubnetMappingChanges, hasSubnetsChanges := diff.HasChange("subnet_mapping"), diff.HasChange(names.AttrSubnets)
	if hasSubnetMappingChanges {
		if v := config.GetAttr("subnet_mapping"); v.IsWhollyKnown() {
			o, n := diff.GetChange("subnet_mapping")
			os, ns := o.(*schema.Set), n.(*schema.Set)

			deltaN := ns.Len() - os.Len()
			switch {
			case deltaN == 0:
				// No change in number of subnet mappings, but one of the mappings did change.
				if err := diff.ForceNew("subnet_mapping"); err != nil {
					return err
				}
			case deltaN < 0:
				// Subnet mappings removed. Ensure that the remaining mappings didn't change.
				if os.Intersection(ns).Len() != ns.Len() {
					if err := diff.ForceNew("subnet_mapping"); err != nil {
						return err
					}
				}
			case deltaN > 0:
				// Subnet mappings added. Ensure that the previous mappings didn't change.
				if ns.Intersection(os).Len() != os.Len() {
					if err := diff.ForceNew("subnet_mapping"); err != nil {
						return err
					}
				}
			}
		}

		if err := diff.SetNewComputed(names.AttrSubnets); err != nil {
			return err
		}
	}
	if hasSubnetsChanges {
		if err := diff.SetNewComputed("subnet_mapping"); err != nil {
			return err
		}
	}

	return nil
}

func customizeDiffLoadBalancerGWLB(_ context.Context, diff *schema.ResourceDiff, v any) error {
	if lbType := awstypes.LoadBalancerTypeEnum(diff.Get("load_balancer_type").(string)); lbType != awstypes.LoadBalancerTypeEnumGateway {
		return nil
	}

	if diff.Id() == "" {
		return nil
	}

	return nil
}

func expandLoadBalancerAccessLogsAttributes(tfMap map[string]any, update bool) []awstypes.LoadBalancerAttribute {
	if tfMap == nil {
		return nil
	}

	var apiObjects []awstypes.LoadBalancerAttribute

	if v, ok := tfMap[names.AttrEnabled].(bool); ok {
		apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
			Key:   aws.String(loadBalancerAttributeAccessLogsS3Enabled),
			Value: flex.BoolValueToString(v),
		})

		if v {
			if v, ok := tfMap[names.AttrBucket].(string); ok && (update || v != "") {
				apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
					Key:   aws.String(loadBalancerAttributeAccessLogsS3Bucket),
					Value: aws.String(v),
				})
			}

			if v, ok := tfMap[names.AttrPrefix].(string); ok && (update || v != "") {
				apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
					Key:   aws.String(loadBalancerAttributeAccessLogsS3Prefix),
					Value: aws.String(v),
				})
			}
		}
	}

	return apiObjects
}

func expandLoadBalancerConnectionLogsAttributes(tfMap map[string]any, update bool) []awstypes.LoadBalancerAttribute {
	if tfMap == nil {
		return nil
	}

	var apiObjects []awstypes.LoadBalancerAttribute

	if v, ok := tfMap[names.AttrEnabled].(bool); ok {
		apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
			Key:   aws.String(loadBalancerAttributeConnectionLogsS3Enabled),
			Value: flex.BoolValueToString(v),
		})

		if v {
			if v, ok := tfMap[names.AttrBucket].(string); ok && (update || v != "") {
				apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
					Key:   aws.String(loadBalancerAttributeConnectionLogsS3Bucket),
					Value: aws.String(v),
				})
			}

			if v, ok := tfMap[names.AttrPrefix].(string); ok && (update || v != "") {
				apiObjects = append(apiObjects, awstypes.LoadBalancerAttribute{
					Key:   aws.String(loadBalancerAttributeConnectionLogsS3Prefix),
					Value: aws.String(v),
				})
			}
		}
	}

	return apiObjects
}

func flattenLoadBalancerAccessLogsAttributes(apiObjects []awstypes.LoadBalancerAttribute) map[string]any {
	if len(apiObjects) == 0 {
		return nil
	}

	tfMap := map[string]any{}

	for _, apiObject := range apiObjects {
		switch k, v := aws.ToString(apiObject.Key), apiObject.Value; k {
		case loadBalancerAttributeAccessLogsS3Enabled:
			tfMap[names.AttrEnabled] = flex.StringToBoolValue(v)
		case loadBalancerAttributeAccessLogsS3Bucket:
			tfMap[names.AttrBucket] = aws.ToString(v)
		case loadBalancerAttributeAccessLogsS3Prefix:
			tfMap[names.AttrPrefix] = aws.ToString(v)
		}
	}

	return tfMap
}

func flattenLoadBalancerConnectionLogsAttributes(apiObjects []awstypes.LoadBalancerAttribute) map[string]any {
	if len(apiObjects) == 0 {
		return nil
	}

	tfMap := map[string]any{}

	for _, apiObject := range apiObjects {
		switch k, v := aws.ToString(apiObject.Key), apiObject.Value; k {
		case loadBalancerAttributeConnectionLogsS3Enabled:
			tfMap[names.AttrEnabled] = flex.StringToBoolValue(v)
		case loadBalancerAttributeConnectionLogsS3Bucket:
			tfMap[names.AttrBucket] = aws.ToString(v)
		case loadBalancerAttributeConnectionLogsS3Prefix:
			tfMap[names.AttrPrefix] = aws.ToString(v)
		}
	}

	return tfMap
}

func expandSubnetMapping(tfMap map[string]any) awstypes.SubnetMapping {
	apiObject := awstypes.SubnetMapping{}

	if v, ok := tfMap["allocation_id"].(string); ok && v != "" {
		apiObject.AllocationId = aws.String(v)
	}

	if v, ok := tfMap["ipv6_address"].(string); ok && v != "" {
		apiObject.IPv6Address = aws.String(v)
	}

	if v, ok := tfMap["private_ipv4_address"].(string); ok && v != "" {
		apiObject.PrivateIPv4Address = aws.String(v)
	}

	if v, ok := tfMap[names.AttrSubnetID].(string); ok && v != "" {
		apiObject.SubnetId = aws.String(v)
	}

	return apiObject
}

func expandSubnetMappings(tfList []any) []awstypes.SubnetMapping {
	if len(tfList) == 0 {
		return nil
	}

	var apiObjects []awstypes.SubnetMapping

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]any)

		if !ok {
			continue
		}

		apiObject := expandSubnetMapping(tfMap)

		apiObjects = append(apiObjects, apiObject)
	}

	return apiObjects
}

func expandIPAMPools(tfList []any) *awstypes.IpamPools {
	if len(tfList) == 0 {
		return nil
	}

	var apiObject awstypes.IpamPools

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]any)

		if !ok {
			continue
		}

		if v, ok := tfMap["ipv4_ipam_pool_id"].(string); ok && v != "" {
			apiObject.Ipv4IpamPoolId = aws.String(v)
		}
	}

	return &apiObject
}

func expandMinimumLoadBalancerCapacity(tfList []any) *awstypes.MinimumLoadBalancerCapacity {
	if len(tfList) == 0 {
		return nil
	}

	var apiObject awstypes.MinimumLoadBalancerCapacity

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]any)

		if !ok {
			continue
		}

		if v, ok := tfMap["capacity_units"].(int); ok && v != 0 {
			apiObject.CapacityUnits = aws.Int32(int32(v))
		}
	}

	return &apiObject
}

func flattenIPAMPools(apiObject *awstypes.IpamPools) []any {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]any{
		"ipv4_ipam_pool_id": aws.ToString(apiObject.Ipv4IpamPoolId),
	}

	return []any{tfMap}
}

func flattenMinimumLoadBalancerCapacity(apiObject *awstypes.MinimumLoadBalancerCapacity) []any {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]any{
		"capacity_units": aws.ToInt32(apiObject.CapacityUnits),
	}

	return []any{tfMap}
}
