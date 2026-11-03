package delivery_log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExplainDSN_KnownCodes(t *testing.T) {
	assert.Contains(t, ExplainDSN("5.1.1"), "does not exist")
	assert.Contains(t, ExplainDSN("5.2.2"), "full")
	assert.Contains(t, ExplainDSN("5.4.4"), "could not be found")
	assert.Contains(t, ExplainDSN(" 5.7.1 "), "blocked", "surrounding spaces are ignored")
}

func TestExplainDSN_FallsBackToClass(t *testing.T) {
	assert.Equal(t, "Delivered", ExplainDSN("2.0.0"))
	assert.Contains(t, ExplainDSN("4.9.9"), "Temporary")
	assert.Contains(t, ExplainDSN("5.9.9"), "Permanent")
}

func TestExplainDSN_Unknown(t *testing.T) {
	assert.Equal(t, "", ExplainDSN(""))
	assert.Equal(t, "", ExplainDSN("abc"))
}
