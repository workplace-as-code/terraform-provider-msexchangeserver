package resources

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// test/attributes_not_changed.txt lists the attributes that can change in place
// but that the lab tests never change. Each release shortens it; no new
// attribute may join it.

const attributesNotChangedFile = "../../test/attributes_not_changed.txt"

// replacesResource reports whether the attribute has a plan modifier that
// recreates the resource. The modifiers of every attribute type share a
// Description method and the field has the same name, hence reflection.
func replacesResource(attribute any) bool {
	field := reflect.ValueOf(attribute).FieldByName("PlanModifiers")
	if !field.IsValid() {
		return false
	}
	for i := 0; i < field.Len(); i++ {
		m, ok := field.Index(i).Interface().(interface {
			Description(context.Context) string
		})
		if ok && strings.Contains(m.Description(context.Background()), "destroy and recreate") {
			return true
		}
	}
	return false
}

// updatableAttributes returns "<type>.<attribute>" for every configurable
// attribute of every registered resource that can change in place.
func updatableAttributes() map[string]bool {
	out := map[string]bool{}
	for _, reg := range registrations() {
		r := reg.newResource()
		var meta resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "msexchangeserver"}, &meta)
		typeName := strings.TrimPrefix(meta.TypeName, "msexchangeserver_")
		var schema resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
		for name, a := range schema.Schema.Attributes {
			if name != "id" && (a.IsOptional() || a.IsRequired()) && !replacesResource(a) {
				out[typeName+"."+name] = true
			}
		}
	}
	return out
}

var (
	localDefinitionRe = regexp.MustCompile(`^\s*(\w+)\s*=\s*(.*)$`)
	localReferenceRe  = regexp.MustCompile(`\blocal\.(\w+)`)
	varReferenceRe    = regexp.MustCompile(`\bvar\.(\w+)`)
	resourceBlockRe   = regexp.MustCompile(`^resource "msexchangeserver_(\w+)" "\w+" \{`)
	// The variables run.ps1 sets for the update phase: -var "name=value" and
	// the variable files it writes as ConvertTo-Json -InputObject @{ name = @{...} }.
	updateVarRe     = regexp.MustCompile(`"-var",\s*"(\w+)=`)
	updateVarFileRe = regexp.MustCompile(`-InputObject @\{ (\w+) = @\{`)
)

// updateVariables returns the variables whose value run.ps1 changes between
// the phases of the run.
func updateVariables(runPs1 string) map[string]bool {
	out := map[string]bool{}
	for _, re := range []*regexp.Regexp{updateVarRe, updateVarFileRe} {
		for _, m := range re.FindAllStringSubmatch(runPs1, -1) {
			out[m[1]] = true
		}
	}
	return out
}

// changedAttributes returns "<type>.<attribute>" for every attribute the lab
// tests change: its value depends on the phase of the run or on a variable
// run.ps1 changes between the phases, in the configuration, directly or
// through a local.
func changedAttributes(tf, runPs1 string) map[string]bool {
	lines := strings.Split(tf, "\n")
	updateVars := updateVariables(runPs1)

	phaseLocals := map[string]bool{}
	dependsOnPhase := func(expr string) bool {
		for _, m := range varReferenceRe.FindAllStringSubmatch(expr, -1) {
			if m[1] == "phase" || updateVars[m[1]] {
				return true
			}
		}
		for _, m := range localReferenceRe.FindAllStringSubmatch(expr, -1) {
			if phaseLocals[m[1]] {
				return true
			}
		}
		return false
	}
	for grown := true; grown; {
		grown = false
		inLocals := false
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "locals {"):
				inLocals = true
			case line == "}":
				inLocals = false
			case inLocals:
				if m := localDefinitionRe.FindStringSubmatch(line); m != nil && !phaseLocals[m[1]] && dependsOnPhase(m[2]) {
					phaseLocals[m[1]] = true
					grown = true
				}
			}
		}
	}

	out := map[string]bool{}
	typeName, attribute := "", ""
	depth := 0
	for _, line := range lines {
		if m := resourceBlockRe.FindStringSubmatch(line); m != nil {
			typeName, attribute, depth = m[1], "", 1
			continue
		}
		if typeName == "" {
			continue
		}
		if depth == 1 {
			attribute = ""
			if m := localDefinitionRe.FindStringSubmatch(line); m != nil {
				attribute = m[1]
			}
		}
		if attribute != "" && dependsOnPhase(line) {
			out[typeName+"."+attribute] = true
		}
		depth += strings.Count(line, "{") + strings.Count(line, "[") + strings.Count(line, "(") -
			strings.Count(line, "}") - strings.Count(line, "]") - strings.Count(line, ")")
		if depth == 0 {
			typeName = ""
		}
	}
	return out
}

// labConfiguration returns the configuration of the Mailbox and Edge suites.
// test/transport is a hand-run test without phases: not counted.
func labConfiguration(t *testing.T) string {
	t.Helper()
	var tf strings.Builder
	for _, pattern := range []string{"../../test/*.tf", "../../test/edge/*.tf"} {
		files, err := filepath.Glob(pattern)
		if err != nil || len(files) == 0 {
			t.Fatalf("no lab configuration matches %s: %v", pattern, err)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			tf.Write(data)
			tf.WriteString("\n")
		}
	}
	return tf.String()
}

func TestAttributesNeverChanged(t *testing.T) {
	tf := labConfiguration(t)
	runPs1, err := os.ReadFile("../../test/run.ps1")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	if data, err := os.ReadFile(attributesNotChangedFile); err == nil {
		for _, l := range strings.Fields(string(data)) {
			listed[l] = true
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	changed := changedAttributes(tf, string(runPs1))
	updatable := updatableAttributes()
	var problems []string
	for a := range updatable {
		if !changed[a] && !listed[a] {
			problems = append(problems, a+": the lab tests never change it")
		}
	}
	for a := range listed {
		switch {
		case !updatable[a]:
			problems = append(problems, a+": listed but it is not an attribute that changes in place")
		case changed[a]:
			problems = append(problems, a+": listed but the lab tests change it, remove it from the list")
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func TestAttributesChangedDetection(t *testing.T) {
	const tf = `
locals {
  updated = var.phase == "update"
  comment = local.updated ? "updated" : "created"
}

resource "msexchangeserver_owa_mailbox_policy" "test" {
  name                    = "TF-CI"
  theme_selection_enabled = local.updated
  blocked_file_types      = local.updated ? ["exe", "bat"] : ["exe"]
}

resource "msexchangeserver_transport_rule" "test" {
  comments = local.comment
}

resource "msexchangeserver_oab_virtual_directory" "test" {
  server               = var.exchange_server
  basic_authentication = var.oab_basic_authentication
}

resource "msexchangeserver_transport_service" "test" {
  connectivity_log_max_age_days = var.settings["transport_service"]
}

resource "msexchangeserver_organization_config" "this" {
  default_authentication_policy = var.texts.default_authentication_policy
}
`
	const runPs1 = `
    $AuthUpdate = @("-var", "oab_basic_authentication=$((-not $oabBasic).ToString().ToLower())")
    $AuthUpdate += @("-var", "settings={$($updated -join ',')}")
    ConvertTo-Json -Compress -InputObject @{ texts = @{ default_authentication_policy = "TF-CI-AuthPolicy" } }
`
	got := changedAttributes(tf, runPs1)
	for _, want := range []string{
		"owa_mailbox_policy.theme_selection_enabled", "owa_mailbox_policy.blocked_file_types",
		"transport_rule.comments", "oab_virtual_directory.basic_authentication",
		"transport_service.connectivity_log_max_age_days", "organization_config.default_authentication_policy",
	} {
		if !got[want] {
			t.Errorf("%s must count as changed, got %v", want, got)
		}
	}
	for _, constant := range []string{"owa_mailbox_policy.name", "oab_virtual_directory.server"} {
		if got[constant] {
			t.Errorf("%s is constant across the phases and must not count as changed", constant)
		}
	}
}
