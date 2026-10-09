package request_test

import (
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/go-playground/validator/v10"
)

func TestRegisterPasswordMinimum(t *testing.T) {
	validate := validator.New()
	register := request.Register{Username: "studio", Email: "studio@example.com", Password: "1234567"}
	if err := validate.Struct(register); err == nil {
		t.Fatal("expected a seven-character password to fail")
	}

	register.Password = "12345678"
	if err := validate.Struct(register); err != nil {
		t.Fatalf("expected an eight-character password to pass: %v", err)
	}
}
