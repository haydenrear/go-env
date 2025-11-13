package lib

import (
	"errors"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server struct {
		Port int
		Host string
	}
	Database struct {
		User     string
		Password string
		Name     string
	}
}

func LoadConfig(configFile string, value any) error {

	if configFile != "" {
		if fi, err := os.Stat(configFile); err == nil && !fi.IsDir() {
			if _, err := toml.DecodeFile(configFile, value); err != nil {
				log.Fatalf("failed to decode config file %q: %v", configFile, err)
			}
			return LoadEnv(value)
		} else if err != nil && !os.IsNotExist(err) {
			log.Fatalf("error checking config file %q: %v", configFile, err)
		} else {
			return LoadEnv(value)
		}
	} else {
		return LoadEnv(value)
	}

	return nil
}

// LoadEnv populates fields of `out` (pointer to struct) that are tagged with `env:"VAR"`.
// It recurses into nested structs and pointer-to-structs. If an env var isn't set, it leaves
// the existing field value as-is.
func LoadEnv(out any) error {
	if out == nil {
		return errors.New("nil target")
	}

	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return errors.New("LoadEnv requires a pointer to a struct")
	}
	return loadInto(rv.Elem())
}

func loadInto(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		// Skip unexported fields.
		if sf.PkgPath != "" {
			continue
		}
		fv := v.Field(i)

		// Always recurse into structs/pointers so we can reach tagged sub-fields.
		switch fv.Kind() {
		case reflect.Struct:
			// Special-case time.Time: treat like scalar, not as a nested struct.
			if fv.Type() == reflect.TypeOf(time.Time{}) {
				if tag := sf.Tag.Get("env"); tag != "" && tag != "-" {
					if s, ok := getenv(tag); ok {
						tm, err := time.Parse(time.RFC3339, s)
						if err != nil {
							return fieldErr(sf, err)
						}
						fv.Set(reflect.ValueOf(tm))
					}
				}
				continue
			}
			if err := loadInto(fv); err != nil {
				return err
			}
			continue
		case reflect.Pointer:
			if fv.Type().Elem().Kind() == reflect.Struct && fv.IsNil() {
				// Allocate so we can populate nested tagged fields.
				fv.Set(reflect.New(fv.Type().Elem()))
			}
			if fv.Type().Elem().Kind() == reflect.Struct {
				if err := loadInto(fv.Elem()); err != nil {
					return err
				}
				continue
			}
		}

		// If there's an env tag on this field, try to set it.
		tag := sf.Tag.Get("env")
		if tag == "" || tag == "-" {
			continue
		}
		if s, ok := getenv(tag); ok {
			if err := setValueFromString(fv, s); err != nil {
				return fieldErr(sf, err)
			}
		}
	}
	return nil
}

func getenv(name string) (string, bool) {
	s, ok := os.LookupEnv(name)
	return s, ok
}

func setValueFromString(v reflect.Value, s string) error {
	// Handle time.Duration explicitly
	if v.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		v.SetInt(int64(d))
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// If the underlying type is time.Time handled earlier; others parse as int.
		i, err := strconv.ParseInt(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.Slice:
		parts := splitComma(s)
		slice := reflect.MakeSlice(v.Type(), len(parts), len(parts))
		for i := range parts {
			if err := setValueFromString(slice.Index(i), parts[i]); err != nil {
				return err
			}
		}
		v.Set(slice)
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setValueFromString(v.Elem(), s)
	default:
		// Unsupported kind; user can extend as needed.
		return errors.New("unsupported field type: " + v.Type().String())
	}
	return nil
}

func splitComma(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func fieldErr(sf reflect.StructField, err error) error {
	return errors.New(sf.Name + ": " + err.Error())
}
