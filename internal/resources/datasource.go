package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Data sources built from resources
//
// A data source reads an existing object by its name and exposes the attributes of its resource, all computed. It runs
// the resource's own ImportState and Read, like terraform query does: the data
// source always reads exactly what the resource reads, and a new attribute of
// the resource appears in both.

// dataSourceResources are the resources that also exist as a data source,
// and the argument that names the object to read. Tenant-wide settings have
// none; accepted domains and the organization config have their own data
// source in internal/datasources.
var dataSourceResources = []dataSourceResource{
	{NewEmailAddressPolicyResource, "name"},
	{NewMobileDeviceMailboxPolicyResource, "name"},
	{NewOwaMailboxPolicyResource, "name"},
	{NewRemoteDomainResource, "name"},
	{NewRetentionPolicyResource, "name"},
	{NewTransportRuleResource, "name"},
}

// dataSourceResource pairs a resource with the argument of its data source,
// whose value is the import ID.
type dataSourceResource struct {
	newResource func() resource.Resource
	key         string
}

// allDataSourceResources adds the generic resources that are not tenant-wide
// settings: they are all imported by name.
func allDataSourceResources() []dataSourceResource {
	out := append([]dataSourceResource(nil), dataSourceResources...)
	for _, spec := range objectSpecs {
		if !spec.singleton {
			out = append(out, dataSourceResource{newObjectResource(spec), "name"})
		}
	}
	return out
}

// DataSources returns the data sources built from resources.
func DataSources() []func() datasource.DataSource {
	sources := allDataSourceResources()
	out := make([]func() datasource.DataSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, func() datasource.DataSource {
			return &resourceDataSource{resource: source.newResource(), key: source.key}
		})
	}
	return out
}

// resourceDataSource is a data source that reads through a resource.
type resourceDataSource struct {
	resource resource.Resource
	key      string
}

var _ datasource.DataSourceWithConfigure = &resourceDataSource{}

func (d *resourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	var meta resource.MetadataResponse
	d.resource.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: req.ProviderTypeName}, &meta)
	resp.TypeName = meta.TypeName
}

// resourceSchema returns the schema of the resource behind the data source.
func (d *resourceDataSource) resourceSchema(ctx context.Context) schema.Schema {
	var resp resource.SchemaResponse
	d.resource.Schema(ctx, resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func (d *resourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var meta datasource.MetadataResponse
	d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "msexchangeserver"}, &meta)
	rs := d.resourceSchema(ctx)
	attrs := make(map[string]dsschema.Attribute, len(rs.Attributes))
	for name, a := range rs.Attributes {
		converted, err := dataSourceAttribute(name == d.key, a)
		if err != nil {
			resp.Diagnostics.AddError("Invalid data source schema", err.Error())
			return
		}
		attrs[name] = converted
	}
	resp.Schema = dsschema.Schema{
		MarkdownDescription: fmt.Sprintf("Reads an existing object by `%[3]s`, with the attributes of the "+
			"[`%[1]s`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/resources/%[2]s) resource. "+
			"Use it to reference an object that this configuration does not manage, such as a default policy.",
			meta.TypeName, meta.TypeName[len("msexchangeserver_"):], d.key),
		Attributes: attrs,
	}
}

// dataSourceAttribute turns a resource attribute into a computed data source
// attribute; the key, which names the object to read, is the only argument.
func dataSourceAttribute(key bool, a schema.Attribute) (dsschema.Attribute, error) {
	desc := a.GetMarkdownDescription()
	if key {
		desc = "Identifies the object to read."
	}
	switch a := a.(type) {
	case schema.StringAttribute:
		return dsschema.StringAttribute{Required: key, Computed: !key, Sensitive: a.Sensitive, MarkdownDescription: desc}, nil
	case schema.BoolAttribute:
		return dsschema.BoolAttribute{Computed: true, Sensitive: a.Sensitive, MarkdownDescription: desc}, nil
	case schema.Int64Attribute:
		return dsschema.Int64Attribute{Computed: true, Sensitive: a.Sensitive, MarkdownDescription: desc}, nil
	case schema.ListAttribute:
		return dsschema.ListAttribute{Computed: true, ElementType: a.ElementType, Sensitive: a.Sensitive, MarkdownDescription: desc}, nil
	case schema.SetAttribute:
		return dsschema.SetAttribute{Computed: true, ElementType: a.ElementType, Sensitive: a.Sensitive, MarkdownDescription: desc}, nil
	}
	return nil, fmt.Errorf("%T has no data source equivalent", a)
}

func (d *resourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configurable, ok := d.resource.(resource.ResourceWithConfigure)
	if !ok {
		return
	}
	var configured resource.ConfigureResponse
	configurable.Configure(ctx, resource.ConfigureRequest{ProviderData: req.ProviderData}, &configured)
	resp.Diagnostics.Append(configured.Diagnostics...)
}

func (d *resourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(d.key), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	importer, ok := d.resource.(resource.ResourceWithImportState)
	if !ok {
		resp.Diagnostics.AddError("Data source not supported", "This resource cannot be imported, so it cannot be read by "+d.key+".")
		return
	}

	// The same steps as an import: an empty state with the import ID, then Read.
	rs := d.resourceSchema(ctx)
	empty := tfsdk.State{Schema: rs, Raw: tftypes.NewValue(rs.Type().TerraformType(ctx), nil)}
	imported := resource.ImportStateResponse{State: empty}
	importer.ImportState(ctx, resource.ImportStateRequest{ID: name.ValueString()}, &imported)
	resp.Diagnostics.Append(imported.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	read := resource.ReadResponse{State: imported.State}
	d.resource.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
	resp.Diagnostics.Append(read.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if read.State.Raw.IsNull() {
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "msexchangeserver"}, &meta)
		resp.Diagnostics.AddError("Object not found", fmt.Sprintf("No %s with %s %q exists in Exchange.", meta.TypeName, d.key, name.ValueString()))
		return
	}
	// Both schemas have the same attributes and types, so the value fits as is.
	resp.State.Raw = read.State.Raw
}
