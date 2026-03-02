package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var addonModelObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"transport_url": types.StringType,
		"name":    types.StringType,
	},
}

type addonModel struct {
	TransportURL types.String `tfsdk:"transport_url"`
	Name    types.String `tfsdk:"name"`
}
