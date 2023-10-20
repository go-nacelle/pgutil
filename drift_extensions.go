package pgutil

import (
	"fmt"
)

type ExtensionModifier struct {
	d ExtensionDescription
}

func NewExtensionModifier(_ SchemaDescription, d ExtensionDescription) ExtensionModifier {
	return ExtensionModifier{
		d: d,
	}
}

func (m ExtensionModifier) Key() string {
	return fmt.Sprintf("%q.%q", m.d.Namespace, m.d.Name)
}

func (m ExtensionModifier) ObjectType() string {
	return "extension"
}

func (m ExtensionModifier) Description() ExtensionDescription {
	return m.d
}

func (m ExtensionModifier) Create() string {
	return fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", m.Key())
}

func (m ExtensionModifier) Drop() string {
	return fmt.Sprintf("DROP EXTENSION IF EXISTS %s;", m.Key())
}
