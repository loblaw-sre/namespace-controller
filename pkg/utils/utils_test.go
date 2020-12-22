package utils_test

import (
	"testing"

	"github.com/loblaw-sre/namespace-controller/pkg/utils"
)

func TestSlug(t *testing.T) {
	actual := utils.Slug("john@loblaw.ca")
	if actual != "john-loblaw-ca" {
		t.Errorf("Slug(\"john@loblaw.ca\") = %s, want john-loblaw-ca", actual)
	}
}
