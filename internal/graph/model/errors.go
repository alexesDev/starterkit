package model

import (
	"errors"
	"sort"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"
)

// NewOzzoError turns a validation failure into the payload the UI renders, one
// message per input field, and returns nil for a nil error so a case can write:
//
//	invalid := model.NewOzzoError(validate(&input))
//	if invalid != nil {
//		return invalid, nil
//	}
func NewOzzoError(err error) *ErrorPayload {
	if err == nil {
		return nil
	}

	var fieldErrors ozzo.Errors
	if !errors.As(err, &fieldErrors) {
		return &ErrorPayload{Message: err.Error()}
	}

	payload := &ErrorPayload{Message: err.Error()}

	for name, fieldErr := range fieldErrors {
		payload.ByFields = append(payload.ByFields, FieldMessage{
			Name:  name,
			Value: fieldErr.Error(),
		})
	}

	sort.Slice(payload.ByFields, func(i, j int) bool {
		return payload.ByFields[i].Name < payload.ByFields[j].Name
	})

	return payload
}
