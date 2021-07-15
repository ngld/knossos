package fso_interop

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/rotisserie/eris"
)

func readUntil(f io.RuneScanner, stop rune) (string, error) {
	buffer := make([]rune, 0, 32)
	for {
		char, _, err := f.ReadRune()
		if err != nil {
			return "", err
		}

		if char == stop {
			return string(buffer), nil
		}

		buffer = append(buffer, char)
	}
}

func skipWhitespace(f io.RuneScanner) error {
	for {
		char, _, err := f.ReadRune()
		if err != nil {
			return err
		}

		switch char {
		case ' ', '\t', '\n', '\r':
			// do nothing
		default:
			return f.UnreadRune()
		}
	}
}

func parseFile(f io.RuneScanner, dest interface{}) error {
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Struct {
		panic("expected dest to be a struct")
	}

	var section reflect.Value
	for {
		char, _, err := f.ReadRune()
		if err != nil {
			if eris.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		switch char {
		case '[':
			label, err := readUntil(f, ']')
			if err != nil {
				return err
			}

			section = destVal.FieldByName(label)
			if !section.IsValid() {
				return eris.Errorf("found unexpected section %s", label)
			}

			err = skipWhitespace(f)
			if err != nil {
				return err
			}
		case '#', ';':
			_, err = readUntil(f, '\n')
			if err != nil {
				return err
			}
		case ' ', '\t', '\n', '\r':
			err = skipWhitespace(f)
			if err != nil {
				return err
			}
		default:
			line, err := readUntil(f, '\n')
			if err != nil {
				return err
			}

			if !section.IsValid() {
				return eris.Errorf("found line \"%s\" before any section", line)
			}

			parts := strings.SplitN(line, "=", 2)
			key := strings.Trim(parts[0], " \r\n\t")
			value := parts[1]
			pos := strings.Index(value, "#")
			if pos > -1 {
				value = value[:pos]
			}

			pos = strings.Index(value, ";")
			if pos > -1 {
				value = value[:pos]
			}

			value = strings.Trim(value, " \r\n\t")

			st := section.Type()
			fieldType, ok := st.FieldByName(key)
			if !ok {
				for idx := 0; idx < st.NumField(); idx++ {
					field := st.Field(idx)
					if field.Tag.Get("ini") == key {
						fieldType = field
						ok = true
						break
					}
				}

				if !ok {
					return eris.Errorf("found unknown key %s", key)
				}
			}

			field := section.FieldByName(fieldType.Name)
			switch field.Type().Kind() {
			case reflect.String:
				field.Set(reflect.ValueOf(value))
			case reflect.Uint32:
				num, err := strconv.Atoi(value)
				if err != nil {
					return eris.Errorf("failed to parse value %s for key %s", value, key)
				}

				field.Set(reflect.ValueOf(num))
			case reflect.Bool:
				num, err := strconv.Atoi(value)
				if err != nil {
					return eris.Errorf("failed to parse value %s for key %s", value, key)
				}

				field.Set(reflect.ValueOf(num > 0))
			default:
				panic(fmt.Sprintf("unexpected type %s for field %s", field.Type().Name(), fieldType.Name))
			}
		}
	}
}
