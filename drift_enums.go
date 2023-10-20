package pgutil

import (
	"fmt"
	"strings"
)

type EnumModifier struct {
	s SchemaDescription // TODO - rename
	d EnumDescription
}

func NewEnumModifier(s SchemaDescription, d EnumDescription) EnumModifier {
	return EnumModifier{
		s: s,
		d: d,
	}
}

func (m EnumModifier) Key() string {
	return fmt.Sprintf("%q.%q", m.d.Namespace, m.d.Name)
}

func (m EnumModifier) ObjectType() string {
	return "enum"
}

func (m EnumModifier) Description() EnumDescription {
	return m.d
}

func (m EnumModifier) Create() string {
	var quotedLabels []string
	for _, label := range m.d.Labels {
		quotedLabels = append(quotedLabels, fmt.Sprintf("'%s'", label)) // TODO - escape '?
	}

	return fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", m.Key(), strings.Join(quotedLabels, ", "))
}

func (m EnumModifier) Drop() string {
	return fmt.Sprintf("DROP TYPE IF EXISTS %s;", m.Key())
}

func (m EnumModifier) AlterExisting(existingSchema SchemaDescription, existingObject EnumDescription) ([]string, bool) {
	reconstruction, ok := unifyLabels(m.d.Labels, existingObject.Labels)
	if !ok {
		var alters []string
		for _, dep := range existingSchema.EnumDependencies {
			if dep.EnumNamespace == m.d.Namespace && dep.EnumName == m.d.Name {
				if false {
					// DROP DEFAULT,
					// Alter...
					// SET DEFAULT ...
				}

				alters = append(alters, fmt.Sprintf(
					"ALTER TABLE %q.%q ALTER COLUMN %q TYPE %s USING (%q::text::%s);",
					dep.TableNamespace,
					dep.TableName,
					dep.ColumnName,
					m.Key(),
					dep.ColumnName,
					m.Key(),
				))
			}
		}

		var stmts []string
		stmts = append(stmts, fmt.Sprintf("ALTER TYPE %q.%q RENAME TO %q;", m.d.Namespace, m.d.Name, m.d.Name+"_bak"))
		stmts = append(stmts, m.Create())
		stmts = append(stmts, alters...)
		stmts = append(stmts, fmt.Sprintf("DROP TYPE %q.%q;", m.d.Namespace, m.d.Name+"_bak"))
		return stmts, true
	}

	var statements []string
	for _, missingLabel := range reconstruction {
		relativeTo := ""
		if missingLabel.Next != nil {
			relativeTo = fmt.Sprintf("BEFORE '%s'", *missingLabel.Next)
		} else {
			relativeTo = fmt.Sprintf("AFTER '%s'", *missingLabel.Prev)
		}

		statements = append(statements, fmt.Sprintf("ALTER TYPE %q.%q ADD VALUE '%s' %s;", m.d.Namespace, m.d.Name, missingLabel.Label, relativeTo))
	}

	return statements, true
}

type missingLabel struct {
	Label string
	Prev  *string
	Next  *string
}

func unifyLabels(expectedLabels, existingLabels []string) (reconstruction []missingLabel, _ bool) {
	var (
		j               = 0
		missingIndexMap = map[int]struct{}{}
	)

	for i, label := range expectedLabels {
		if j < len(existingLabels) && existingLabels[j] == label {
			j++
		} else if i > 0 {
			missingIndexMap[i] = struct{}{}
		}
	}

	if j < len(existingLabels) {
		return nil, false
	}

	if expectedLabels[0] != existingLabels[0] {
		reconstruction = append(reconstruction, missingLabel{
			Label: expectedLabels[0],
			Next:  &existingLabels[0],
		})
	}

	for i, label := range expectedLabels {
		if _, ok := missingIndexMap[i]; ok {
			reconstruction = append(reconstruction, missingLabel{
				Label: label,
				Prev:  &expectedLabels[i-1],
			})
		}
	}

	return reconstruction, true
}
