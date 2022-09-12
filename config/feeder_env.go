package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/halorium/env"

	"kraftkit.sh/utils"
)

type EnvFeeder struct{}

func (f EnvFeeder) Feed(structure interface{}) error {
	err := env.Unmarshal(*structure.(**Config))

	var obj AuthConfig
	rv := reflect.ValueOf(obj)

	cfg := *structure.(**Config)
	cfg.Auth = make(map[string]AuthConfig)
	for i := 0; i < rv.NumField(); i++ {

		rsf := rv.Type().Field(i)
		tag := rsf.Tag.Get("env")

		if !strings.Contains(tag, "%s") {
			continue
		}

		prefix := strings.Split(tag, "%s")[0]
		suffix := strings.Split(tag, "%s")[1]

		envVars := utils.Filter(os.Environ(), func(s string) bool {
			return strings.HasPrefix(s, prefix) &&
				strings.HasSuffix(strings.Split(s, "=")[0], suffix)
		})

		for _, s := range envVars {
			index := utils.GetStringInBetween(s, prefix, suffix)
			index = strings.ToLower(index)

			entry, exists := cfg.Auth[index]
			if !exists {
				entry = *new(AuthConfig)
			}

			authRv := reflect.ValueOf(&entry).Elem()
			for j := 0; j < authRv.NumField(); j++ {
				authRsf := authRv.Type().Field(j)
				authRf := authRv.Field(j)
				authTag := authRsf.Tag.Get("env")

				stringValue := strings.Split(s, "=")[1]

				if strings.HasSuffix(strings.Split(s, "=")[0],
					strings.Split(authTag, "%s")[1]) {

					switch authRf.Type().Kind() {
					case reflect.String:
						authRf.SetString(stringValue)
					case reflect.Bool:
						val, err := strconv.ParseBool(stringValue)
						if err != nil {
							return err
						}
						authRf.SetBool(val)
					case reflect.Int:
						val, err := strconv.ParseInt(stringValue, 0, 32)
						if err != nil {
							return err
						}
						authRf.SetInt(val)
					}
				}
			}
			cfg.Auth[index] = entry

		}
	}

	return err
}

func (f EnvFeeder) Write(structure interface{}, merge bool) error {
	return nil
}
