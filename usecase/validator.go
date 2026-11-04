package usecase

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/laddawan/backend-test/model"
)

// Validator validates use-case inputs and converts failures into
// *model.ValidationError so every transport reports them the same way.
type Validator struct {
	v *validator.Validate
}

// NewValidator creates a Validator. Field names in errors are lower-cased
// struct field names (e.g. "email").
func NewValidator() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		return strings.ToLower(f.Name)
	})
	return &Validator{v: v}
}

// Struct validates s using its `validate` struct tags.
func (val *Validator) Struct(s any) error {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return fmt.Errorf("validate input: %w", err)
	}

	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fe.Field()] = message(fe)
	}
	return &model.ValidationError{Fields: fields}
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	default:
		return "is invalid"
	}
}
