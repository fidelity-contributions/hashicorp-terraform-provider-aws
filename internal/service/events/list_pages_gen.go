// Code generated by "internal/generate/listpages/main.go -ListOps=ListApiDestinations,ListArchives,ListConnections,ListEventBuses,ListEventSources,ListRules,ListTargetsByRule"; DO NOT EDIT.

package events

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

func listAPIDestinationsPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListApiDestinationsInput, fn func(*eventbridge.ListApiDestinationsOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListApiDestinations(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listArchivesPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListArchivesInput, fn func(*eventbridge.ListArchivesOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListArchives(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listConnectionsPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListConnectionsInput, fn func(*eventbridge.ListConnectionsOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListConnections(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listEventBusesPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListEventBusesInput, fn func(*eventbridge.ListEventBusesOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListEventBuses(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listEventSourcesPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListEventSourcesInput, fn func(*eventbridge.ListEventSourcesOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListEventSources(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listRulesPages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListRulesInput, fn func(*eventbridge.ListRulesOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListRules(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
func listTargetsByRulePages(ctx context.Context, conn *eventbridge.Client, input *eventbridge.ListTargetsByRuleInput, fn func(*eventbridge.ListTargetsByRuleOutput, bool) bool, optFns ...func(*eventbridge.Options)) error {
	for {
		output, err := conn.ListTargetsByRule(ctx, input, optFns...)
		if err != nil {
			return err
		}

		lastPage := aws.ToString(output.NextToken) == ""
		if !fn(output, lastPage) || lastPage {
			break
		}

		input.NextToken = output.NextToken
	}
	return nil
}
