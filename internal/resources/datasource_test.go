package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// TestDataSourcesMirrorTheirResource checks every data source built from a
// resource: its key is the only argument, and its value type is the
// resource's, since Read copies the resource state as is.
func TestDataSourcesMirrorTheirResource(t *testing.T) {
	ctx := context.Background()
	if len(DataSources()) != len(allDataSourceResources()) {
		t.Fatalf("%d data sources for %d resources", len(DataSources()), len(allDataSourceResources()))
	}
	for _, newDataSource := range DataSources() {
		d := newDataSource().(*resourceDataSource)
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "msexchangeserver"}, &meta)
		t.Run(meta.TypeName, func(t *testing.T) {
			var resp datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			for name, a := range resp.Schema.Attributes {
				if a.IsRequired() != (name == d.key) || (name != d.key && !a.IsComputed()) {
					t.Errorf("%s: required=%v computed=%v", name, a.IsRequired(), a.IsComputed())
				}
			}
			if _, ok := resp.Schema.Attributes[d.key].(dsschema.StringAttribute); !ok {
				t.Errorf("no %s attribute", d.key)
			}
			want := d.resourceSchema(ctx).Type().TerraformType(ctx)
			if got := resp.Schema.Type().TerraformType(ctx); !got.Equal(want) {
				t.Errorf("type differs from the resource:\n%s\n%s", got, want)
			}
		})
	}
}
