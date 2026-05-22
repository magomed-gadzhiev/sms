package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/osteele/liquid"
)

// ValidationResult holds the result of template validation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// LengthEstimate holds estimated output length info.
type LengthEstimate struct {
	MinLen   int
	MaxLen   int
	Segments int
	Warning  string
}

// Validator validates Liquid templates for SMS use.
type Validator struct {
	engine *liquid.Engine
}

// NewValidator creates a new template Validator.
func NewValidator() *Validator {
	return &Validator{engine: liquid.NewEngine()}
}

// Validate checks template syntax.
func (v *Validator) Validate(templateStr string) ValidationResult {
	_, err := v.engine.ParseString(templateStr)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return ValidationResult{Valid: true}
}

var variableRegex = regexp.MustCompile(`\{\{[\s]*([a-zA-Z_][a-zA-Z0-9_]*)`)
var tagVarRegex = regexp.MustCompile(`\{%[\s]*(?:if|unless|for)[\s]+([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExtractVariables returns all variable names referenced in the template.
func (v *Validator) ExtractVariables(templateStr string) []string {
	seen := map[string]bool{}
	var vars []string

	for _, matches := range variableRegex.FindAllStringSubmatch(templateStr, -1) {
		name := matches[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	for _, matches := range tagVarRegex.FindAllStringSubmatch(templateStr, -1) {
		name := matches[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

// EstimateLength estimates the min/max output length given sample values per variable.
func (v *Validator) EstimateLength(templateStr string, sampleValues map[string][]interface{}) *LengthEstimate {
	renderer := NewRenderer()

	minLen := int(^uint(0) >> 1) // max int
	maxLen := 0

	// Render with each combination of shortest and longest values
	shortBindings := map[string]interface{}{}
	longBindings := map[string]interface{}{}

	for varName, values := range sampleValues {
		if len(values) == 0 {
			continue
		}
		shortest := values[0]
		longest := values[0]
		for _, val := range values {
			s := fmt.Sprintf("%v", val)
			if len(s) < len(fmt.Sprintf("%v", shortest)) {
				shortest = val
			}
			if len(s) > len(fmt.Sprintf("%v", longest)) {
				longest = val
			}
		}
		shortBindings[varName] = shortest
		longBindings[varName] = longest
	}

	// Render with short values
	if out, err := renderer.Render(templateStr, shortBindings); err == nil {
		l := len([]rune(out))
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}

	// Render with long values
	if out, err := renderer.Render(templateStr, longBindings); err == nil {
		l := len([]rune(out))
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}

	if minLen == int(^uint(0)>>1) {
		minLen = 0
	}

	segments := (maxLen + 159) / 160
	if segments < 1 {
		segments = 1
	}

	var warning string
	if segments > 1 {
		warning = fmt.Sprintf("Максимальная длина %d симв. (%d SMS-сегментов)", maxLen, segments)
	}

	return &LengthEstimate{
		MinLen:   minLen,
		MaxLen:   maxLen,
		Segments: segments,
		Warning:  warning,
	}
}

// ValidateAttributes checks that all variables in the template exist in the given attribute schema.
func (v *Validator) ValidateAttributes(templateStr string, availableAttrs []string) []string {
	vars := v.ExtractVariables(templateStr)
	attrSet := make(map[string]bool, len(availableAttrs))
	for _, a := range availableAttrs {
		attrSet[a] = true
	}
	attrSet["name"] = true
	attrSet["phone"] = true

	var missing []string
	for _, varName := range vars {
		if !attrSet[varName] && !isBuiltinVariable(varName) {
			missing = append(missing, varName)
		}
	}
	return missing
}

func isBuiltinVariable(name string) bool {
	builtins := map[string]bool{
		"true": true, "false": true, "nil": true, "null": true,
		"blank": true, "empty": true,
	}
	return builtins[strings.ToLower(name)]
}
