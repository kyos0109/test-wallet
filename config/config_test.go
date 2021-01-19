package config

import (
	"testing"
)

func TestReadConfig(t *testing.T) {
	_, err := ReadConfig("./none_file.yaml")

	if err != nil {
		t.Log("ok")
	} else {
		t.Fail()
	}
}
