package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// registration pairs a resource with the query that lists its objects.
type registration struct {
	newResource func() resource.Resource
	list        listQuery
	edge        bool // also available on Edge Transport servers (role = "Edge")
}

// registrations returns every resource of the provider. A new resource is
// added here (generic resources are added to objectSpecs instead).
func registrations() []registration {
	regs := []registration{
		// Edge Transport servers have their own transport rules, and keep a
		// copy of the accepted and remote domains (synchronized by EdgeSync
		// when the server is subscribed).
		{NewTransportRuleResource, listQuery{noun: "TransportRule"}, true},
		{NewRemoteDomainResource, listQuery{noun: "RemoteDomain"}, true},
		{NewAcceptedDomainResource, listQuery{noun: "AcceptedDomain"}, true},
		{NewOwaMailboxPolicyResource, listQuery{noun: "OwaMailboxPolicy"}, false},
		{NewOrganizationConfigResource, listQuery{id: "organization"}, false},
		{NewEmailAddressPolicyResource, listQuery{noun: "EmailAddressPolicy"}, false},
		{NewMobileDeviceMailboxPolicyResource, listQuery{noun: "MobileDeviceMailboxPolicy"}, false},
		{NewRetentionPolicyResource, listQuery{noun: "RetentionPolicy"}, false},
	}
	for _, spec := range objectSpecs {
		regs = append(regs, registration{newObjectResource(spec), spec.listQuery(), spec.edge})
	}
	return regs
}

// Resources returns the resources registered in the provider, with their identity.
func Resources() []func() resource.Resource {
	regs := registrations()
	out := make([]func() resource.Resource, 0, len(regs))
	for _, reg := range regs {
		out = append(out, withIdentity(reg.newResource, reg.edge))
	}
	return out
}

// ListResources returns the list resource of every resource, for terraform query.
func ListResources() []func() list.ListResource {
	regs := registrations()
	out := make([]func() list.ListResource, 0, len(regs))
	for _, reg := range regs {
		out = append(out, newListResource(reg.newResource, reg.list))
	}
	return out
}
