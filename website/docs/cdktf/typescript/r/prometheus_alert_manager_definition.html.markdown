---
subcategory: "AMP (Managed Prometheus)"
layout: "aws"
page_title: "AWS: aws_prometheus_alert_manager_definition"
description: |-
  Manages an Amazon Managed Service for Prometheus (AMP) Alert Manager Definition
---


<!-- Please do not edit this file, it is generated. -->
# Resource: aws_prometheus_alert_manager_definition

Manages an Amazon Managed Service for Prometheus (AMP) Alert Manager Definition

## Example Usage

```typescript
// DO NOT EDIT. Code generated by 'cdktf convert' - Please report bugs at https://cdk.tf/bug
import { Construct } from "constructs";
import { TerraformStack } from "cdktf";
/*
 * Provider bindings are generated by running `cdktf get`.
 * See https://cdk.tf/provider-generation for more details.
 */
import { PrometheusAlertManagerDefinition } from "./.gen/providers/aws/prometheus-alert-manager-definition";
import { PrometheusWorkspace } from "./.gen/providers/aws/prometheus-workspace";
class MyConvertedCode extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);
    const demo = new PrometheusWorkspace(this, "demo", {});
    const awsPrometheusAlertManagerDefinitionDemo =
      new PrometheusAlertManagerDefinition(this, "demo_1", {
        definition:
          "alertmanager_config: |\n  route:\n    receiver: 'default'\n  receivers:\n    - name: 'default'\n\n",
        workspaceId: demo.id,
      });
    /*This allows the Terraform resource name to match the original name. You can remove the call if you don't need them to match.*/
    awsPrometheusAlertManagerDefinitionDemo.overrideLogicalId("demo");
  }
}

```

## Argument Reference

This resource supports the following arguments:

* `workspaceId` - (Required) ID of the prometheus workspace the alert manager definition should be linked to
* `definition` - (Required) the alert manager definition that you want to be applied. See more [in AWS Docs](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-alert-manager.html).

## Attribute Reference

This resource exports no additional attributes.

## Import

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import the prometheus alert manager definition using the workspace identifier. For example:

```typescript
// DO NOT EDIT. Code generated by 'cdktf convert' - Please report bugs at https://cdk.tf/bug
import { Construct } from "constructs";
import { TerraformStack } from "cdktf";
/*
 * Provider bindings are generated by running `cdktf get`.
 * See https://cdk.tf/provider-generation for more details.
 */
import { PrometheusAlertManagerDefinition } from "./.gen/providers/aws/prometheus-alert-manager-definition";
class MyConvertedCode extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);
    PrometheusAlertManagerDefinition.generateConfigForImport(
      this,
      "demo",
      "ws-C6DCB907-F2D7-4D96-957B-66691F865D8B"
    );
  }
}

```

Using `terraform import`, import the prometheus alert manager definition using the workspace identifier. For example:

```console
% terraform import aws_prometheus_alert_manager_definition.demo ws-C6DCB907-F2D7-4D96-957B-66691F865D8B
```

<!-- cache-key: cdktf-0.20.8 input-6a4a52d7d4acb917f5c7ab057a46d8d09f6db1143f23bbfd0bbd195ab2a7b439 -->