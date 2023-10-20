package pgutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnifyLabels(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		expectedLabels []string
		existingLabels []string
		valid          bool
		reconstruction []missingLabel
	}{
		{
			name:           "additional",
			expectedLabels: []string{"foo", "bar", "baz"},
			existingLabels: []string{"foo", "baz", "bonk"},
			valid:          false,
		},
		{
			name:           "inversions",
			expectedLabels: []string{"foo", "bar", "baz"},
			existingLabels: []string{"baz", "bar"},
			valid:          false,
		},

		{
			name:           "missing at end",
			expectedLabels: []string{"foo", "bar", "baz", "bonk"},
			existingLabels: []string{"foo", "bar"},
			valid:          true,
			reconstruction: []missingLabel{
				{Label: "baz", Prev: ptr("bar")},
				{Label: "bonk", Prev: ptr("baz")},
			},
		},
		{
			name:           "missing in middle",
			expectedLabels: []string{"foo", "bar", "baz", "bonk"},
			existingLabels: []string{"foo", "bonk"},
			valid:          true,
			reconstruction: []missingLabel{
				{Label: "bar", Prev: ptr("foo")},
				{Label: "baz", Prev: ptr("bar")},
			},
		},
		{
			name:           "missing at beginning",
			expectedLabels: []string{"foo", "bar", "baz", "bonk"},
			existingLabels: []string{"baz", "bonk"},
			valid:          true,
			reconstruction: []missingLabel{
				{Label: "foo", Next: ptr("baz")},
				{Label: "bar", Prev: ptr("foo")},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if reconstruction, valid := unifyLabels(testCase.expectedLabels, testCase.existingLabels); valid {
				if !testCase.valid {
					t.Fatalf("expected unification to fail")
				} else if diff := cmp.Diff(testCase.reconstruction, reconstruction); diff != "" {
					t.Errorf("unexpected reconstruction (-want +got):\n%s", diff)
				}
			} else if testCase.valid {
				t.Fatalf("expected unification to succeed")
			}
		})
	}
}

func ptr[S any](v S) *S {
	return &v
}
