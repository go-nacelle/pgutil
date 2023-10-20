package pgutil

import (
	"cmp"
	"fmt"

	"golang.org/x/exp/slices"
)

func Compare(a, b SchemaDescription) (statements []string) {
	var (
		aTableComponentModifiers = NewTableComponentModifiers(a, a.Tables)
		bTableComponentModifiers = NewTableComponentModifiers(b, b.Tables)
	)

	createExtensionStatements, dropExtensionStatements, replaceExtensionStatements := compareObjects(a, b, wrapWithContextValue(a, a.Extensions, NewExtensionModifier), wrapWithContextValue(b, b.Extensions, NewExtensionModifier))
	createEnumStatements, dropEnumStatements, replaceEnumStatements := compareObjects(a, b, wrapWithContextValue(a, a.Enums, NewEnumModifier), wrapWithContextValue(b, b.Enums, NewEnumModifier))
	createFunctionStatements, dropFunctionStatements, replaceFunctionStatements := compareObjects(a, b, wrapWithContextValue(a, a.Functions, NewFunctionModifier), wrapWithContextValue(b, b.Functions, NewFunctionModifier))
	createTableStatements, dropTableStatements, replaceTableStatements := compareObjects(a, b, wrapWithContextValue(a, a.Tables, NewTableModifier), wrapWithContextValue(b, b.Tables, NewTableModifier))
	createSequenceStatements, dropSequenceStatements, replaceSequenceStatements := compareObjects(a, b, wrapWithContextValue(a, a.Sequences, NewSequenceModifier), wrapWithContextValue(b, b.Sequences, NewSequenceModifier))
	createColumnStatements, dropColumnStatements, replaceColumnStatements := compareObjects(a, b, aTableComponentModifiers.Columns, bTableComponentModifiers.Columns)
	createConstraintStatements, dropConstraintStatements, replaceConstraintStatements := compareObjects(a, b, aTableComponentModifiers.Constraints, bTableComponentModifiers.Constraints)
	createIndexStatements, dropIndexStatements, replaceIndexStatements := compareObjects(a, b, aTableComponentModifiers.Indexes, bTableComponentModifiers.Indexes)
	createViewStatements, dropViewStatements := compareViews(a, b)
	createTriggerStatements, dropTriggerStatements, replaceTriggerStatements := compareObjects(a, b, wrapWithContextValue(a, a.Triggers, NewTriggerModifier), wrapWithContextValue(b, b.Triggers, NewTriggerModifier))

	objectStatements := []struct {
		create  []ddlStatement
		drop    []ddlStatement
		replace []ddlStatement
	}{
		{createExtensionStatements, dropExtensionStatements, replaceExtensionStatements},
		{createEnumStatements, dropEnumStatements, replaceEnumStatements},
		{createFunctionStatements, dropFunctionStatements, replaceFunctionStatements},
		{createTableStatements, dropTableStatements, replaceTableStatements},
		{createSequenceStatements, dropSequenceStatements, replaceSequenceStatements},
		{createColumnStatements, dropColumnStatements, replaceColumnStatements},
		{createConstraintStatements, dropConstraintStatements, replaceConstraintStatements},
		{createIndexStatements, dropIndexStatements, replaceIndexStatements},
		{createViewStatements, dropViewStatements, nil},
		{createTriggerStatements, dropTriggerStatements, replaceTriggerStatements},
	}

	for i := len(objectStatements) - 1; i >= 0; i-- {
		for _, ddlStatement := range objectStatements[i].drop {
			statements = append(statements, ddlStatement.statements...)
		}
	}
	for _, objectStatement := range objectStatements {
		for _, ddlStatement := range objectStatement.replace {
			statements = append(statements, ddlStatement.statements...)
		}
	}
	for _, objectStatement := range objectStatements {
		for _, ddlStatement := range objectStatement.create {
			statements = append(statements, ddlStatement.statements...)
		}
	}

	//
	//
	//

	// Dependency mapping:
	//
	// extensions  : no dependencies
	// enums       : no dependencies
	// functions   : no dependencies
	// tables      : no dependencies
	// sequences   : no dependencies
	// columns     : depends on tables, enums, sequences
	// constraints : depends on tables, columns
	// indexes     : depends on tables, columns
	// views       : depends on tables, columns, views
	// triggers    : depends on tables, columns, functions

	return statements
}

//
//
//

func compareViews(a, b SchemaDescription) (create, drop []ddlStatement) {
	create, drop, _ = compareObjects(
		a,
		b,
		wrapWithContextValue(a, a.Views, NewViewModifier),
		wrapWithContextValue(b, b.Views, NewViewModifier),
	)

	createKeys := map[string]struct{}{}
	for _, statement := range create {
		createKeys[statement.key] = struct{}{}
	}

	createClosure := closure{}
	for _, dependency := range a.ColumnDependencies {
		sourceKey := fmt.Sprintf("%q.%q", dependency.SourceNamespace, dependency.SourceTableOrViewName)
		dependencyKey := fmt.Sprintf("%q.%q", dependency.UsedNamespace, dependency.UsedTableOrView)

		if _, ok := createKeys[sourceKey]; ok {
			if _, ok := createClosure[sourceKey]; !ok {
				createClosure[sourceKey] = map[string]struct{}{}
			}

			createClosure[sourceKey][dependencyKey] = struct{}{}
		}
	}
	transitiveClosure(createClosure)
	slices.SortFunc(create, cmpWithClosure(createClosure))

	dropKeys := map[string]struct{}{}
	for _, statement := range drop {
		dropKeys[statement.key] = struct{}{}
	}

	dropClosure := closure{}
	for _, dependency := range b.ColumnDependencies {
		sourceKey := fmt.Sprintf("%q.%q", dependency.SourceNamespace, dependency.SourceTableOrViewName)
		dependencyKey := fmt.Sprintf("%q.%q", dependency.UsedNamespace, dependency.UsedTableOrView)

		if _, ok := dropKeys[dependencyKey]; ok {
			if _, ok := dropClosure[dependencyKey]; !ok {
				dropClosure[dependencyKey] = map[string]struct{}{}
			}

			dropClosure[dependencyKey][sourceKey] = struct{}{}
		}
	}
	transitiveClosure(dropClosure)
	slices.SortFunc(drop, cmpWithClosure(dropClosure))

	return create, drop
}

//
//

type closure map[string]map[string]struct{}

func transitiveClosure(m closure) {
	changed := true
	for changed {
		changed = false

		for _, m2 := range m {
			for k := range m2 {
				for v := range m[k] {
					if _, ok := m2[v]; !ok {
						m2[v] = struct{}{}
						changed = true
					}
				}
			}
		}
	}
}

func cmpWithClosure(createClosure closure) func(a, b ddlStatement) int {
	return func(a, b ddlStatement) int {
		if _, ok := createClosure[a.key][b.key]; ok {
			return -1
		}
		if _, ok := createClosure[b.key][a.key]; ok {
			return +1
		}

		return cmp.Compare(a.key, b.key)
	}
}

//
//
//

type keyer interface {
	Key() string
}

type equaler[T any] interface {
	Equals(T) bool
}

type modifier[T equaler[T]] interface {
	keyer
	ObjectType() string
	Description() T
	Create() string
	Drop() string
}

type alterer[T any] interface {
	AlterExisting(existingSchema SchemaDescription, existingObject T) ([]string, bool)
}

type ddlStatement struct {
	key        string
	objectType string // TODO - enum
	statements []string
}

func newStatement(key string, objectType string, statements ...string) ddlStatement {
	return ddlStatement{
		key:        key,
		objectType: objectType,
		statements: statements,
	}
}

func compareObjects[T equaler[T], M modifier[T]](a, b SchemaDescription, as, bs []M) (create, drop, replace []ddlStatement) {
	missing, additional, common := partition(as, bs)

	for _, modifier := range missing {
		create = append(create, newStatement(
			modifier.Key(),
			modifier.ObjectType(),
			modifier.Create(),
		))
	}

	for _, modifier := range additional {
		drop = append(drop, newStatement(
			modifier.Key(),
			modifier.ObjectType(),
			modifier.Drop(),
		))
	}

	for _, pair := range common {
		var (
			aModifier    = pair.a
			bModifier    = pair.b
			aDescription = aModifier.Description()
			bDescription = bModifier.Description()
		)

		if aDescription.Equals(bDescription) {
			continue
		}

		if alterer, ok := any(aModifier).(alterer[T]); ok {
			if alterStatements, ok := alterer.AlterExisting(b, bDescription); ok {
				replace = append(replace, newStatement(
					aModifier.Key(),
					aModifier.ObjectType(),
					alterStatements...,
				))

				continue
			}
		}

		drop = append(drop, newStatement(bModifier.Key(), bModifier.ObjectType(), bModifier.Drop()))
		create = append(create, newStatement(aModifier.Key(), aModifier.ObjectType(), aModifier.Create()))
	}

	return create, drop, replace
}

//
//
//

type pair[T any] struct {
	a, b T
}

// missing = present in a but not b
// additional = present in b but not a
func partition[T keyer](a, b []T) (missing, additional []T, common []pair[T]) {
	aMap := map[string]T{}
	for _, value := range a {
		aMap[value.Key()] = value
	}

	bMap := map[string]T{}
	for _, value := range b {
		bMap[value.Key()] = value
	}

	for key, aValue := range aMap {
		if bValue, ok := bMap[key]; ok {
			common = append(common, pair[T]{aValue, bValue})
		} else {
			missing = append(missing, aValue)
		}
	}

	for key, bValue := range bMap {
		if _, ok := aMap[key]; !ok {
			additional = append(additional, bValue)
		}
	}

	return missing, additional, common
}

//
//
//

func wrap[T, R any](s []T, f func(T) R) (wrapped []R) {
	for _, value := range s {
		wrapped = append(wrapped, f(value))
	}

	return wrapped
}

func wrapWithContextValue[C, T, R any](c C, s []T, f func(C, T) R) []R {
	return wrap(s, func(v T) R { return f(c, v) })
}

func wrapWithContextValues[C1, C2, T, R any](c1 C1, c2 C2, s []T, f func(C1, C2, T) R) []R {
	return wrap(s, func(v T) R { return f(c1, c2, v) })
}
