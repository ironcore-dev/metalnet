// Copyright 2022 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sysfs

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

const (
	TagKey = "sysfs"
)

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "sysfs: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "sysfs: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "sysfs: Unmarshal(nil " + e.Type.String() + ")"
}

type Unmarshaler interface {
	Unmarshal(filename string) error
}

func unmarshal(filename string, v any, rv reflect.Value) error {
	if unmarshaler, ok := v.(Unmarshaler); ok {
		return unmarshaler.Unmarshal(filename)
	}

	// TODO: Extend these cases.
	elemType := rv.Type().Elem()
	switch elemType.Kind() {
	case reflect.Uint64:
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		res, err := strconv.ParseUint(strings.TrimSpace(string(data)), 0, 64)
		if err != nil {
			return err
		}

		rv.Elem().SetUint(res)
		return nil
	case reflect.Struct:
		for i := 0; i < elemType.NumField(); i++ {
			field := elemType.Field(i)

			name, ok := field.Tag.Lookup(TagKey)
			if !ok {
				name = field.Name
			}

			childFilename := filepath.Join(filename, name)
			dst := rv.Elem().Field(i).Addr()
			if err := unmarshal(childFilename, dst.Interface(), dst); err != nil {
				return err
			}
		}
		return nil
	default:
		return &InvalidUnmarshalError{rv.Type()}
	}
}

func Unmarshal(filename string, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	return unmarshal(filename, v, rv)
}
