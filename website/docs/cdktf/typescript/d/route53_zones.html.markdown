---
subcategory: "Route 53"
layout: "aws"
page_title: "AWS: aws_route53_zones"
description: |-
    Provides a list of Route53 Hosted Zone IDs in a Region
---


<!-- Please do not edit this file, it is generated. -->
# Data Source: aws_route53_zones

This resource can be useful for getting back a list of Route53 Hosted Zone IDs for a Region.

## Example Usage

The following example retrieves a list of all Hosted Zone IDs.

```typescript
// DO NOT EDIT. Code generated by 'cdktf convert' - Please report bugs at https://cdk.tf/bug
import { Construct } from "constructs";
import { TerraformOutput, TerraformStack } from "cdktf";
/*
 * Provider bindings are generated by running `cdktf get`.
 * See https://cdk.tf/provider-generation for more details.
 */
import { DataAwsRoute53Zones } from "./.gen/providers/aws/data-aws-route53-zones";
class MyConvertedCode extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);
    const all = new DataAwsRoute53Zones(this, "all", {});
    new TerraformOutput(this, "example", {
      value: all.ids,
    });
  }
}

```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `ids` - A list of all the Route53 Hosted Zone IDs found.

<!-- cache-key: cdktf-0.20.8 input-45d160bcdac0615de23bc94f24c3ec3d8307857791439a4341824b2ac9d936dc -->