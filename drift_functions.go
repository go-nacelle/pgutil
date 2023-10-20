package pgutil

import (
	"fmt"
)

type FunctionModifier struct {
	d FunctionDescription
}

func NewFunctionModifier(_ SchemaDescription, d FunctionDescription) FunctionModifier {
	return FunctionModifier{
		d: d,
	}
}

func (m FunctionModifier) Key() string {
	return fmt.Sprintf("%q.%q", m.d.Namespace, m.d.Name)
}

func (m FunctionModifier) ObjectType() string {
	return "function"
}

func (m FunctionModifier) Description() FunctionDescription {
	return m.d
}

func (m FunctionModifier) Create() string {
	return fmt.Sprintf("%s;", m.d.Definition)
}

func (m FunctionModifier) AlterExisting(_ SchemaDescription, _ FunctionModifier) ([]string, bool) {
	return []string{m.Create()}, true
}

func (m FunctionModifier) Drop() string {
	// TODO - capture args
	return fmt.Sprintf("DROP FUNCTION IF EXISTS %s;", m.Key())
}
