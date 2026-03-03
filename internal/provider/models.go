package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var addonModelObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"addon_id":      types.StringType,
		"transport_url": types.StringType,
		"name":          types.StringType,
		"version":       types.StringType,
		"types":         types.ListType{ElemType: types.StringType},
		"catalog_types": types.ListType{ElemType: types.StringType},
		"resources":     types.ListType{ElemType: types.StringType},
	},
}

type addonModel struct {
	AddonID      types.String `tfsdk:"addon_id"`
	TransportURL types.String `tfsdk:"transport_url"`
	Name         types.String `tfsdk:"name"`
	Version      types.String `tfsdk:"version"`
	Types        types.List   `tfsdk:"types"`
	CatalogTypes types.List   `tfsdk:"catalog_types"`
	Resources    types.List   `tfsdk:"resources"`
}
