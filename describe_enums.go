package pgutil

import (
	"context"

	"golang.org/x/exp/slices"
)

type EnumDescription struct {
	Namespace string
	Name      string
	Labels    []string
}

func (d EnumDescription) Equals(other EnumDescription) bool {
	return true &&
		d.Namespace == other.Namespace &&
		d.Name == other.Name &&
		slices.Equal(d.Labels, other.Labels)
}

type enum struct {
	Namespace string
	Name      string
	Label     string
}

var scanEnums = NewSliceScanner(func(s Scanner) (l enum, _ error) {
	err := s.Scan(&l.Namespace, &l.Name, &l.Label)
	return l, err
})

func DescribeEnums(ctx context.Context, db DB) ([]EnumDescription, error) {
	enumLabels, err := scanEnums(db.Query(ctx, RawQuery(`
		SELECT
			n.nspname AS namespace,
			t.typname AS name,
			e.enumlabel AS label
		FROM pg_catalog.pg_enum e
		JOIN pg_catalog.pg_type t ON t.oid = e.enumtypid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		ORDER BY n.nspname, t.typname, e.enumsortorder
	`)))
	if err != nil {
		return nil, err
	}

	var enums []EnumDescription
	for _, enumLabel := range enumLabels {
		if l := last(enums); l.Namespace != enumLabel.Namespace || l.Name != enumLabel.Name {
			enums = append(enums, EnumDescription{
				Namespace: enumLabel.Namespace,
				Name:      enumLabel.Name,
				Labels:    nil,
			})
		}

		n := len(enums)
		enums[n-1].Labels = append(enums[n-1].Labels, enumLabel.Label)
	}

	return enums, nil
}

func last[T any](s []T) (last T) {
	if n := len(s); n > 0 {
		last = s[n-1]
	}

	return last
}
