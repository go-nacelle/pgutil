package pgutil

import (
	"fmt"
)

type SequenceModifier struct {
	d SequenceDescription
}

func NewSequenceModifier(_ SchemaDescription, d SequenceDescription) SequenceModifier {
	return SequenceModifier{
		d: d,
	}
}

func (m SequenceModifier) Key() string {
	return fmt.Sprintf("%q.%q", m.d.Namespace, m.d.Name)
}

func (m SequenceModifier) ObjectType() string {
	return "sequence"
}

func (m SequenceModifier) Description() SequenceDescription {
	return m.d
}

func (m SequenceModifier) Create() string {
	minValue := "NO MINVALUE"
	if m.d.MinimumValue != 0 {
		minValue = fmt.Sprintf("MINVALUE %d", m.d.MinimumValue)
	}

	maxValue := "NO MAXVALUE"
	if m.d.MaximumValue != 0 {
		maxValue = fmt.Sprintf("MAXVALUE %d", m.d.MaximumValue)
	}

	return fmt.Sprintf(
		"CREATE SEQUENCE IF NOT EXISTS %s AS %s INCREMENT BY %d %s %s START WITH %d %s CYCLE;",
		m.Key(),
		m.d.Type,
		m.d.Increment,
		minValue,
		maxValue,
		m.d.StartValue,
		m.d.CycleOption,
	)
}

func (m SequenceModifier) Drop() string {
	return fmt.Sprintf("DROP SEQUENCE IF EXISTS %s;", m.Key())
}

// TODO - alter

// statements := []string{}

// if d.TypeName != target.TypeName {
// 	statements = append(statements, fmt.Sprintf("ALTER SEQUENCE %s AS %s MAXVALUE %d;", d.Name, target.TypeName, target.MaximumValue))

// 	// Remove from diff below
// 	d.TypeName = target.TypeName
// 	d.MaximumValue = target.MaximumValue
// }

// // Abort if there are other fields we haven't addressed
// hasAdditionalDiff := cmp.Diff(d, target) != ""
// return statements, !hasAdditionalDiff
