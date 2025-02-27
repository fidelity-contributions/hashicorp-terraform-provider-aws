---
subcategory: "Elemental MediaStore"
layout: "aws"
page_title: "AWS: aws_media_store_container"
description: |-
  Provides a MediaStore Container.
---


<!-- Please do not edit this file, it is generated. -->
# Resource: aws_media_store_container

Provides a MediaStore Container.

## Example Usage

```typescript
// DO NOT EDIT. Code generated by 'cdktf convert' - Please report bugs at https://cdk.tf/bug
import { Construct } from "constructs";
import { TerraformStack } from "cdktf";
/*
 * Provider bindings are generated by running `cdktf get`.
 * See https://cdk.tf/provider-generation for more details.
 */
import { MediaStoreContainer } from "./.gen/providers/aws/media-store-container";
class MyConvertedCode extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);
    new MediaStoreContainer(this, "example", {
      name: "example",
    });
  }
}

```

## Argument Reference

This resource supports the following arguments:

* `name` - (Required) The name of the container. Must contain alphanumeric characters or underscores.
* `tags` - (Optional) A map of tags to assign to the resource. If configured with a provider [`defaultTags` configuration block](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#default_tags-configuration-block) present, tags with matching keys will overwrite those defined at the provider-level.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - The ARN of the container.
* `endpoint` - The DNS endpoint of the container.
* `tagsAll` - A map of tags assigned to the resource, including those inherited from the provider [`defaultTags` configuration block](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#default_tags-configuration-block).

## Import

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import MediaStore Container using the MediaStore Container Name. For example:

```typescript
// DO NOT EDIT. Code generated by 'cdktf convert' - Please report bugs at https://cdk.tf/bug
import { Construct } from "constructs";
import { TerraformStack } from "cdktf";
/*
 * Provider bindings are generated by running `cdktf get`.
 * See https://cdk.tf/provider-generation for more details.
 */
import { MediaStoreContainer } from "./.gen/providers/aws/media-store-container";
class MyConvertedCode extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);
    MediaStoreContainer.generateConfigForImport(this, "example", "example");
  }
}

```

Using `terraform import`, import MediaStore Container using the MediaStore Container Name. For example:

```console
% terraform import aws_media_store_container.example example
```

<!-- cache-key: cdktf-0.20.8 input-534cdde6fc1c98559d9e005ae9cd5a252af07161d62318424cfe7000b0a2d854 -->