// Package generator builds Kubernetes CRD manifests from parsed proto messages
// and writes them as YAML files.
package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/iglin/envoy-mesh/crd-gen/internal/parser"
	"github.com/iglin/envoy-mesh/crd-gen/internal/schema"
)

// blockedMessages are proto message FQNs that will never be generated as CRDs.
// Bootstrap is excluded because it is a proxy startup config, not an xDS resource.
var blockedMessages = map[string]bool{
	"envoy.config.bootstrap.v3.Bootstrap": true,
}

// Config holds CLI parameters forwarded from the cobra command.
type Config struct {
	ProtoDir string
	OutDir   string
	Group    string
	Version  string
	Messages []string // fully-qualified proto message names
	// KindExtraProps injects additional top-level schema properties per CRD kind.
	// Key is the proto message short name (e.g. "Cluster").
	// Ranging over a nil map is safe in Go and produces zero iterations.
	KindExtraProps map[string]map[string]apiextensionsv1.JSONSchemaProps
}

// Run is the main entry point: parses protos, generates CRDs, writes YAML.
func Run(cfg Config) error {
	reg, err := parser.ParseDir(cfg.ProtoDir)
	if err != nil {
		return fmt.Errorf("parsing proto directory %q: %w", cfg.ProtoDir, err)
	}
	fmt.Printf("Parsed %d messages and %d enums from %s\n",
		len(reg.Messages), len(reg.Enums), cfg.ProtoDir)

	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %q: %w", cfg.OutDir, err)
	}

	for _, fqn := range cfg.Messages {
		if blockedMessages[fqn] {
			fmt.Printf("  ✗  %s  (blocked — skipped)\n", fqn)
			continue
		}
		msg, ok := reg.Messages[fqn]
		if !ok {
			return fmt.Errorf("message %q not found in proto directory — check the fully-qualified name", fqn)
		}

		crd := buildCRD(msg, cfg, reg)

		data, err := marshalCRD(crd)
		if err != nil {
			return fmt.Errorf("marshaling CRD for %s: %w", fqn, err)
		}

		plural := strings.ToLower(msg.Name) + "s"
		outPath := filepath.Join(cfg.OutDir, plural+"."+cfg.Group+".yaml")
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
		fmt.Printf("  ✓  %s  →  %s\n", fqn, outPath)
	}
	return nil
}

func buildCRD(msg *parser.Message, cfg Config, reg *parser.Registry) *apiextensionsv1.CustomResourceDefinition {
	kind := msg.Name
	plural := strings.ToLower(kind) + "s"
	singular := strings.ToLower(kind)

	specSchema := schema.MessageToSchema(msg, reg)

	topLevelProps := map[string]apiextensionsv1.JSONSchemaProps{
		"targetRef": {
			Type:        "object",
			Description: "Reference to the EnvoyProxy CR that this resource is assigned to.",
			Required:    []string{"name"},
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"name":      {Type: "string", Description: "Name of the EnvoyProxy CR."},
				"namespace": {Type: "string", Description: "Namespace of the EnvoyProxy CR. Defaults to the namespace of this resource."},
			},
		},
		"spec":   *specSchema,
		"status": {Type: "object", XPreserveUnknownFields: boolPtr(true)},
	}
	for k, v := range cfg.KindExtraProps[kind] {
		topLevelProps[k] = v
	}

	topLevelSchema := &apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Required:   []string{"targetRef"},
		Properties: topLevelProps,
	}

	return &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: plural + "." + cfg.Group,
			Annotations: map[string]string{
				"mesh.iglin.io/proto-source": msg.FullName,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: cfg.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     kind,
				ListKind: kind + "List",
				Plural:   plural,
				Singular: singular,
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    cfg.Version,
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: topLevelSchema,
					},
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}
}

func marshalCRD(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	data, err := yaml.Marshal(crd)
	if err != nil {
		return nil, err
	}
	// Prepend a document separator for clarity when multiple CRDs are concatenated.
	return append([]byte("---\n"), data...), nil
}

func boolPtr(b bool) *bool { return &b }

func float64Ptr(f float64) *float64 { return &f }
