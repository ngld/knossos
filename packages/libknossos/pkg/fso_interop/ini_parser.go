package fso_interop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/rotisserie/eris"
)

func readUntil(f io.RuneScanner, stop rune) (string, error) {
	buffer := make([]rune, 0, 32)
	for {
		char, _, err := f.ReadRune()
		if err != nil {
			return "", eris.Wrap(err, "failed to read rune")
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
			if eris.Is(err, io.EOF) {
				return err
			}
			return eris.Wrap(err, "failed to read rune")
		}

		switch char {
		case ' ', '\t', '\n', '\r':
			// do nothing
		default:
			err = f.UnreadRune()
			if err != nil {
				return eris.Wrap(err, "failed to queue rune back")
			}

			return nil
		}
	}
}

func parseFile(f io.RuneScanner, dest interface{}) error {
	destVal := reflect.ValueOf(dest).Elem()
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
			return eris.Wrap(err, "failed to read rune")
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

			if section.IsNil() {
				section.Set(reflect.New(section.Type().Elem()))
			}
			section = section.Elem()

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
				if eris.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		default:
			err = f.UnreadRune()
			if err != nil {
				return eris.Wrap(err, "failed to push rune back on stack")
			}

			line, err := readUntil(f, '\n')
			if err != nil {
				if eris.Is(err, io.EOF) {
					return nil
				}
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
					if strings.SplitN(field.Tag.Get("json"), ",", 2)[0] == key {
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

				field.Set(reflect.ValueOf(uint32(num)))
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

func LoadSettings(ctx context.Context) (*client.FSOSettings, error) {
	iniPath := filepath.Join(GetPrefPath(ctx), "fs2_open.ini")
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to read %s", iniPath)
	}

	buffer := strings.NewReader(string(data))
	var settings client.FSOSettings
	// assign defaults

	settings.Default = &client.FSOSettings_DefaultSettings{
		GammaD3D:      "1.0",
		Language:      "English",
		SpeechVolume:  100,
		TextureFilter: 1,
	}
	settings.Sound = &client.FSOSettings_SoundSettings{
		SampleRate: "441000",
	}
	settings.ForceFeedback = &client.FSOSettings_ForceFeedbackSettings{
		Strength: 100,
	}

	err = parseFile(buffer, &settings)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to parse %s", iniPath)
	}

	return &settings, nil
}

func SaveSettings(ctx context.Context, settings *client.FSOSettings) error {
	buffer := strings.Builder{}
	value := reflect.ValueOf(settings).Elem()
	settingsType := value.Type()

	for idx := 0; idx < settingsType.NumField(); idx++ {
		sectionField := settingsType.Field(idx)
		if !sectionField.IsExported() {
			continue
		}

		sectionValues := value.Field(idx).Elem()
		if !sectionValues.IsValid() {
			api.Log(ctx, api.LogWarn, "Couldn't read %s", sectionField.Name)
			continue
		}

		if idx > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(fmt.Sprintf("[%s]\n", sectionField.Name))

		sectionType := sectionValues.Type()
		for f := 0; f < sectionType.NumField(); f++ {
			field := sectionType.Field(f)
			if !field.IsExported() {
				continue
			}

			buffer.WriteString(field.Name)
			buffer.WriteString("=")

			switch value := sectionValues.Field(f).Interface().(type) {
			case string:
				buffer.WriteString(value)
			case int32, uint32:
				buffer.WriteString(fmt.Sprintf("%d", value))
			case bool:
				if value {
					buffer.WriteString("1")
				} else {
					buffer.WriteString("0")
				}
			default:
				return eris.Errorf("discovered unsupported type %s in field %s in section %s", sectionValues.Field(f).String(), field.Name, sectionField.Name)
			}
			buffer.WriteString("\n")
		}
	}

	iniPath := filepath.Join(GetPrefPath(ctx), "fs2_open.ini")
	err := os.WriteFile(iniPath, []byte(buffer.String()), 0600)
	if err != nil {
		return eris.Wrapf(err, "failed to write %s", iniPath)
	}

	return nil
}
