package contractsapi

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
)

// fieldMappingInfo holds information about how a QueryResult column maps to a struct field.
// It's unexported as it's an internal detail of DecodeCallResult.
type fieldMappingInfo struct {
	StructFieldIndex int
	// Potentially add OriginalColumnName string if needed for detailed error reporting
}

// createScannedItemInternal handles the creation of the final typed item (T)
// from a reflect.Value container that was populated by ScanTo.
// originalType is the reflect.Type of T itself (e.g., *MyStruct or MyStruct).
// scannedValueContainer is typically the result of reflect.New(elementType), which is a pointer.
func createScannedItemInternal[T any](originalType reflect.Type, scannedValueContainer reflect.Value) T {
	var finalValue T
	if originalType.Kind() == reflect.Ptr {
		// If T is a pointer type (*MyStruct), scannedValueContainer is *MyStruct (or **MyStruct if element type was already a pointer, though reflect.New(elementType) usually gives one level of pointer)
		// We need to ensure scannedValueContainer actually is of type T.
		if scannedValueContainer.Type() == originalType {
			finalValue = scannedValueContainer.Interface().(T)
		} else if scannedValueContainer.Elem().Type() == originalType.Elem() && scannedValueContainer.Type().Kind() == reflect.Ptr {
			// This case might occur if T is *S and scannedValueContainer is **S, and we want *S
			// However, typical usage with reflect.New(elementType) where elementType is S means scannedValueContainer is *S.
			// Let's assume scannedValueContainer is directly assignable or its element is.
			finalValue = scannedValueContainer.Interface().(T) // This will panic if type T is *S and scannedValueContainer is S. Ok, because reflect.New gives *S.
		} else {
			// Fallback, should generally be hit if originalType is *S and scannedValueContainer is *S
			finalValue = scannedValueContainer.Interface().(T)
		}
	} else {
		// If T is a non-pointer type (MyStruct), scannedValueContainer is *MyStruct.
		// We need to dereference it to get MyStruct.
		finalValue = scannedValueContainer.Elem().Interface().(T)
	}
	return finalValue
}

// mapColumnsToStructFieldsInternal maps QueryResult column names to struct field indices.
// structElemType is the reflect.Type of the struct itself (not a pointer to it).
func mapColumnsToStructFieldsInternal(structElemType reflect.Type, columnNames []string) ([]*fieldMappingInfo, error) {
	numCols := len(columnNames)
	colDestinations := make([]*fieldMappingInfo, numCols)

	for colIdx, colName := range columnNames {
		foundFieldForColumn := false
		for fieldIdx := 0; fieldIdx < structElemType.NumField(); fieldIdx++ {
			field := structElemType.Field(fieldIdx)

			if field.PkgPath != "" { // Field is unexported
				continue
			}

			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" { // Field is explicitly ignored
				continue
			}

			tagKey := ""
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if len(parts) > 0 && parts[0] != "" {
					tagKey = parts[0]
				}
			}

			nameMatch := false
			if tagKey != "" {
				if colName == tagKey {
					nameMatch = true
				}
			} else {
				// Fallback to field name match (case-insensitive for first char, then sensitive, or just equal fold)
				if colName == field.Name || strings.EqualFold(colName, field.Name) {
					nameMatch = true
				}
			}

			if nameMatch {
				colDestinations[colIdx] = &fieldMappingInfo{StructFieldIndex: fieldIdx}
				foundFieldForColumn = true
				break // Found mapping for this column, move to next column
			}
		}
		_ = foundFieldForColumn // Can be used for logging if a column doesn't map to any field
	}
	return colDestinations, nil
}

// prepareScanTargetsForStructInternal prepares the []any slice of destination pointers for ScanTo.
// structInstanceVal is the reflect.Value of the struct instance (not a pointer to it).
// structElemType is the reflect.Type of the struct, for error messages.
// mappings is the output from mapColumnsToStructFieldsInternal.
func prepareScanTargetsForStructInternal(structInstanceVal reflect.Value, structElemType reflect.Type, mappings []*fieldMappingInfo) ([]any, error) {
	numCols := len(mappings)
	dstArgs := make([]any, numCols)

	for colIdx := 0; colIdx < numCols; colIdx++ {
		if targetInfo := mappings[colIdx]; targetInfo != nil {
			fieldInStruct := structInstanceVal.Field(targetInfo.StructFieldIndex)
			if !fieldInStruct.CanAddr() {
				// This should ideally not happen for exported fields of a struct obtained via reflect.New().Elem()
				return nil, errors.Errorf("cannot address field %s (index %d) in struct %s", structElemType.Field(targetInfo.StructFieldIndex).Name, targetInfo.StructFieldIndex, structElemType.Name())
			}
			dstArgs[colIdx] = fieldInStruct.Addr().Interface() // Pointer to field
		} else {
			// For columns in QueryResult that don't map to any field in T, scan into a dummy var.
			dstArgs[colIdx] = new(any) // Or use &sql.RawBytes{} or similar if specific discard behavior is needed
		}
	}
	return dstArgs, nil
}

// DecodeCallResult decodes the result of a view call to a slice of T.
// T is typically a struct type, where fields are mapped from QueryResult columns
// using json tags or field names.
// If T is a scalar type (e.g. int, string) and QueryResult has a single column,
// it decodes each row's single value into T.
func DecodeCallResult[T any](result *kwiltypes.QueryResult) ([]T, error) {
	if result == nil {
		return nil, errors.New("QueryResult is nil")
	}

	if len(result.Values) == 0 {
		return []T{}, nil
	}

	var sampleT T
	originalTypeOfT := reflect.TypeOf(sampleT)

	elementType := originalTypeOfT
	if elementType != nil && elementType.Kind() == reflect.Ptr {
		elementType = elementType.Elem()
	}

	if elementType == nil { // Can happen if T is an interface type that is nil (e.g. var sampleT any)
		return nil, errors.New("type T is nil or an uninitialized interface")
	}

	// Handle scalar type T if QueryResult has a single column
	if elementType.Kind() != reflect.Struct && elementType.Kind() != reflect.Map { // Maps are not directly supported by ScanTo for field mapping
		if len(result.ColumnNames) == 1 {
			var scalarResults = make([]T, 0, len(result.Values))
			for _, rowSrc := range result.Values {
				if len(rowSrc) != 1 {
					return nil, errors.Errorf("expected single value in row for scalar decoding, got %d values for type %s", len(rowSrc), elementType.Name())
				}

				// Create a new instance of the underlying element type of T to scan into (e.g., *string, *int)
				itemContainer := reflect.New(elementType)

				err := kwiltypes.ScanTo([]any{rowSrc[0]}, itemContainer.Interface())
				if err != nil {
					return nil, errors.Wrapf(err, "failed to scan scalar value into %s", elementType.Name())
				}

				finalValue := createScannedItemInternal[T](originalTypeOfT, itemContainer)
				scalarResults = append(scalarResults, finalValue)
			}
			return scalarResults, nil
		}
		return nil, errors.Errorf("DecodeCallResult: T must be a struct, or a scalar type with a single-column QueryResult. Got type %s (element type %s) with %d columns", originalTypeOfT.Name(), elementType.Name(), len(result.ColumnNames))
	}

	// T is a struct type (or pointer to struct)
	numCols := len(result.ColumnNames)
	if numCols == 0 && len(result.Values) > 0 && len(result.Values[0]) > 0 {
		return nil, errors.New("QueryResult has data rows but no column names, cannot map to struct")
	}

	mappings, err := mapColumnsToStructFieldsInternal(elementType, result.ColumnNames)
	if err != nil {
		// mapColumnsToStructFieldsInternal currently doesn't return an error, but good practice for future.
		return nil, errors.Wrap(err, "failed to map columns to struct fields")
	}

	var newResults = make([]T, 0, len(result.Values))
	for _, rowSrc := range result.Values {
		if len(rowSrc) != numCols {
			return nil, errors.Errorf("row length %d does not match column count %d for type %s", len(rowSrc), numCols, elementType.Name())
		}

		// Create a new instance of the struct (e.g., *MyStruct)
		itemContainer := reflect.New(elementType)
		// Get the actual struct value (MyStruct) to access its fields
		itemStructVal := itemContainer.Elem()

		dstArgs, err := prepareScanTargetsForStructInternal(itemStructVal, elementType, mappings)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to prepare scan targets for struct %s", elementType.Name())
		}

		err = kwiltypes.ScanTo(rowSrc, dstArgs...)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to scan row into struct %s", elementType.Name())
		}

		finalValue := createScannedItemInternal[T](originalTypeOfT, itemContainer)
		newResults = append(newResults, finalValue)
	}

	return newResults, nil
}
