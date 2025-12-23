package util

// TransformOrNil returns nil if the value is nil, otherwise applies the transform function.
//
// This helper is commonly used when building argument arrays for SQL procedure calls
// where optional parameters should be represented as SQL NULL.
//
// Example:
//
//	args = append(args, util.TransformOrNil(input.From, func(date int) any { return date }))
//	args = append(args, util.TransformOrNil(input.To, func(date int) any { return date }))
func TransformOrNil[T any](value *T, transform func(T) any) any {
	if value == nil {
		return nil
	}
	return transform(*value)
}
