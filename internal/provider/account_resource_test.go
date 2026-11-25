package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestAccountEmailRequiresReplace(t *testing.T) {
	t.Parallel()

	r := NewAccountResource()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema: %v", resp.Diagnostics)
	}

	email, ok := resp.Schema.Attributes["email"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatal("email attribute missing from account schema")
	}
	for _, modifier := range email.PlanModifiers {
		if strings.Contains(modifier.Description(context.Background()), "destroy and recreate") {
			return
		}
	}
	t.Fatal("email attribute must RequireReplace so changing it replaces the account")
}
