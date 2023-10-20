package pgutil

type ScanValueFunc[T any] func(Scanner) (T, error)
type MaybeScanValueFunc[T any] func(Scanner) (T, bool, error)

func newMaybeScanValueFunc[T any](f ScanValueFunc[T]) MaybeScanValueFunc[T] {
	return func(s Scanner) (T, bool, error) {
		value, err := f(s)
		return value, true, err
	}
}

func NewAnyValueScanner[T any]() ScanValueFunc[T] {
	return func(s Scanner) (value T, err error) {
		err = s.Scan(&value)
		return
	}
}
