package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVariables_SingleVar(t *testing.T) {
	vars := ExtractVariables("Hello {{name}}!")
	assert.Equal(t, []string{"name"}, vars)
}

func TestExtractVariables_MultipleVars(t *testing.T) {
	vars := ExtractVariables("Hi {{first_name}} {{last_name}}, your code is {{code}}.")
	assert.Equal(t, []string{"first_name", "last_name", "code"}, vars)
}

func TestExtractVariables_Duplicates(t *testing.T) {
	// Duplicate occurrences of the same variable should appear only once.
	vars := ExtractVariables("{{greeting}} {{name}}, {{greeting}} again!")
	assert.Equal(t, []string{"greeting", "name"}, vars)
}

func TestExtractVariables_NoVars(t *testing.T) {
	vars := ExtractVariables("No placeholders here.")
	assert.Nil(t, vars)
}

func TestExtractVariables_EmptyString(t *testing.T) {
	vars := ExtractVariables("")
	assert.Nil(t, vars)
}

func TestExtractVariables_UnderscoredNames(t *testing.T) {
	vars := ExtractVariables("{{first_name}} {{last_name}} {{promo_code_2}}")
	assert.Equal(t, []string{"first_name", "last_name", "promo_code_2"}, vars)
}

func TestExtractVariables_PreservesOrder(t *testing.T) {
	vars := ExtractVariables("{{z}} {{a}} {{m}}")
	assert.Equal(t, []string{"z", "a", "m"}, vars)
}
