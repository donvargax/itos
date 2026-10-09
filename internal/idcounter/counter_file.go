package idcounter

// replaceFileWith separates path conversion from the platform's atomic move
// so both conversion failures and the replacement call have a testable seam.
func replaceFileWith[T any](source, destination string, convert func(string) (T, error), move func(T, T, uint32) error, flags uint32) error {
	from, err := convert(source)
	if err != nil {
		return err
	}
	to, err := convert(destination)
	if err != nil {
		return err
	}
	return move(from, to, flags)
}
