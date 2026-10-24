package resources

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FlexStringSlice unmarshals either a JSON string or a JSON array of strings.
// PowerShell's ConvertTo-Json collapses single-element arrays to scalars in some versions,
// so the bridge may return "domain.com" instead of ["domain.com"]. This type handles both.
type FlexStringSlice []string

func (s *FlexStringSlice) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if single == "" {
			*s = FlexStringSlice{}
		} else {
			*s = FlexStringSlice{single}
		}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*s = FlexStringSlice(arr)
	return nil
}

// listToCSV serializes a types.List into a comma-separated string for PowerShell
func listToCSV(ctx context.Context, list types.List) string {
	if list.IsNull() || list.IsUnknown() {
		return ""
	}
	var items []string
	list.ElementsAs(ctx, &items, false)
	return strings.Join(items, ",")
}

// stringsToList converts a Go string slice into a Terraform types.List
func stringsToList(items []string) types.List {
	if len(items) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}
	vals := make([]attr.Value, len(items))
	for i, s := range items {
		vals[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, vals)
}

// stringsToSet converts a Go string slice into a Terraform types.Set (order-independent)
func stringsToSet(items []string) types.Set {
	if len(items) == 0 {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}
	vals := make([]attr.Value, len(items))
	for i, s := range items {
		vals[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, vals)
}

// setToCSV serializes a types.Set into a comma-separated string for PowerShell
func setToCSV(ctx context.Context, set types.Set) string {
	if set.IsNull() || set.IsUnknown() {
		return ""
	}
	var items []string
	set.ElementsAs(ctx, &items, false)
	return strings.Join(items, ",")
}
