package pgutil

import "fmt"

type ColumnModifier struct {
	t TableDescription
	d ColumnDescription
}

func NewColumnModifier(_ SchemaDescription, t TableDescription, d ColumnDescription) ColumnModifier {
	return ColumnModifier{
		t: t,
		d: d,
	}
}

func (m ColumnModifier) Key() string {
	return fmt.Sprintf("%q.%q.%q", m.t.Namespace, m.t.Name, m.d.Name)
}

func (m ColumnModifier) ObjectType() string {
	return "column"
}

func (m ColumnModifier) Description() ColumnDescription {
	return m.d
}

func (m ColumnModifier) Create() string {
	nullableExpr := ""
	if !m.d.IsNullable {
		nullableExpr = " NOT NULL"
	}

	defaultExpr := ""
	if m.d.Default != "" {
		defaultExpr = fmt.Sprintf(" DEFAULT %s", m.d.Default)
	}

	return fmt.Sprintf("ALTER TABLE %q.%q ADD COLUMN IF NOT EXISTS %q %s %s %s;", m.t.Namespace, m.t.Name, m.d.Name, m.d.Type, nullableExpr, defaultExpr)
}

func (m ColumnModifier) Drop() string {
	return fmt.Sprintf("ALTER TABLE %q.%q DROP COLUMN IF EXISTS %q;", m.t.Namespace, m.t.Name, m.d.Name)
}

func (m ColumnModifier) AlterExisting(existingSchema SchemaDescription, existingObject ColumnDescription) ([]string, bool) {
	// TODO - stop tracking?
	// Index                  int

	// TODO - implement these
	// Type                   string
	// IsNullable             bool
	// Default                string

	// TODO - how to modify these?
	// CharacterMaximumLength int
	// IsIdentity             bool
	// IdentityGeneration     string
	// IsGenerated            bool
	// GenerationExpression   string

	return nil, false
}
