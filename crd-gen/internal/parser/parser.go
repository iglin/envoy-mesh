// Package parser walks a proto source tree and builds a flat registry of all
// message and enum definitions, keyed by their fully-qualified proto names.
package parser

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
)

// Field represents a single proto field (normal, map, or oneof member).
type Field struct {
	Name     string
	Type     string // proto scalar, or (possibly relative) message/enum name
	Repeated bool
	// Set only for map fields.
	MapKey   string
	MapValue string
	Comment  string
}

// Message represents a parsed proto message definition.
type Message struct {
	FullName string // e.g. "envoy.config.listener.v3.Listener"
	Name     string // simple name, e.g. "Listener"
	Package  string // proto package, e.g. "envoy.config.listener.v3"
	Fields   []*Field
	Comment  string
}

// Enum represents a parsed proto enum definition.
type Enum struct {
	FullName string
	Values   []string
}

// Registry holds every message and enum discovered across all parsed files.
type Registry struct {
	Messages map[string]*Message // fullName → Message
	Enums    map[string]*Enum    // fullName → Enum
}

// ParseDir walks dir recursively and parses every .proto file it finds.
// Files that cannot be parsed are silently skipped.
func ParseDir(dir string) (*Registry, error) {
	reg := &Registry{
		Messages: make(map[string]*Message),
		Enums:    make(map[string]*Enum),
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return err
		}
		parseFile(path, reg)
		return nil
	})
	return reg, err
}

func parseFile(path string, reg *Registry) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	def, err := proto.NewParser(f).Parse()
	if err != nil {
		return // skip files that do not parse cleanly
	}

	var pkg string
	for _, elem := range def.Elements {
		if p, ok := elem.(*proto.Package); ok {
			pkg = p.Name
			break
		}
	}

	collectElements(def.Elements, pkg, "", reg)
}

// collectElements recursively collects messages and enums from a list of proto
// elements. parentFQN is non-empty when processing nested types.
func collectElements(elements []proto.Visitee, pkg, parentFQN string, reg *Registry) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *proto.Message:
			fqn := qualify(e.Name, pkg, parentFQN)
			msg := &Message{
				FullName: fqn,
				Name:     e.Name,
				Package:  pkg,
				Comment:  commentText(e.Comment),
			}
			for _, el := range e.Elements {
				switch f := el.(type) {
				case *proto.NormalField:
					msg.Fields = append(msg.Fields, &Field{
						Name:     f.Name,
						Type:     f.Type,
						Repeated: f.Repeated,
						Comment:  commentText(f.Comment),
					})
				case *proto.MapField:
					msg.Fields = append(msg.Fields, &Field{
						Name:     f.Name,
						Type:     "map",
						MapKey:   f.KeyType,
						MapValue: f.Field.Type,
					})
				case *proto.Oneof:
					for _, oe := range f.Elements {
						if of, ok := oe.(*proto.OneOfField); ok {
							msg.Fields = append(msg.Fields, &Field{
								Name:    of.Name,
								Type:    of.Type,
								Comment: commentText(of.Comment),
							})
						}
					}
				}
			}
			reg.Messages[fqn] = msg
			// Recurse for nested types.
			collectElements(e.Elements, pkg, fqn, reg)

		case *proto.Enum:
			fqn := qualify(e.Name, pkg, parentFQN)
			enum := &Enum{FullName: fqn}
			for _, el := range e.Elements {
				if ev, ok := el.(*proto.EnumField); ok {
					enum.Values = append(enum.Values, ev.Name)
				}
			}
			reg.Enums[fqn] = enum
		}
	}
}

// qualify builds the fully-qualified name for a type given its simple name,
// the file's package, and the enclosing message's FQN (if any).
func qualify(name, pkg, parentFQN string) string {
	if parentFQN != "" {
		return parentFQN + "." + name
	}
	if pkg != "" {
		return pkg + "." + name
	}
	return name
}

func commentText(c *proto.Comment) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(c.Lines, " "))
}
