// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadSupervisorConfig(t *testing.T) {
	var testIniFile = os.Getenv("TEST_INI_FILE")
	if testIniFile != "" {
		data, err := os.ReadFile(testIniFile)
		assert.Nil(t, err)

		conf, err := ReadSupervisorConfig(data)
		assert.Nil(t, err)

		bindata, _ := json.Marshal(conf)
		t.Logf("supervisord conf is: %s", bindata)

		var buf bytes.Buffer
		err = WriteSupervisorConfig(conf, &buf)
		assert.Nil(t, err)
		t.Logf("supervisord ini is: %s", buf.Bytes())
	}
}
