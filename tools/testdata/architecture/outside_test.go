package architecture_test

import (
	"strings"
	"testing"
)

// checkOutside runs architecture-check --outside over TestArchitecture's
// fixtures: cmd importing the slices accepted, every other package outside
// internal/slices importing one refused, from its code or its tests, but
// what a row of the exceptions allows, which the fixtures set through
// ARCHITECTURE_OUTSIDE_EXCEPTIONS.
func checkOutside(t *testing.T, check, dir string, env []string, module string) {
	row := "internal/git tests internal/slices/alpha/domain"
	for _, c := range []struct {
		pkg, imported, from, exceptions string
	}{
		{"./cmd/slices", "", "", ""},
		{"./internal/git/internal_slices_alpha_allowed_code", "internal/slices/alpha", "", ""},
		// A store serves every slice through the assembly and imports none.
		{"./internal/store/private/adapter/internal_slices_alpha_code", "internal/slices/alpha", "", ""},
		{"./internal/store/tree/adapter/internal_slices_alpha_code", "internal/slices/alpha", "", ""},
		{"./internal/store/tracker/adapter/internal_slices_alpha_code", "internal/slices/alpha", "", ""},
		// An engine imports no slice: a slice calls it.
		{"./internal/engine/alpha/internal_slices_alpha_code", "internal/slices/alpha", "", ""},
		{"./internal/cli/internal_slices_alpha_domain_allowed_test", "internal/slices/alpha/domain", " from its tests", ""},
		{"./internal/git/internal_slices_alpha_domain_allowed_test", "internal/slices/alpha/domain", " from its tests", ""},
		// The row excepts git's tests, its subpackages' with them, and
		// nothing else.
		{"./internal/git/internal_slices_alpha_domain_allowed_test", "", "", row},
		{"./internal/git/code/internal_slices_alpha_domain_allowed_code", "internal/slices/alpha/domain", "", row},
		{"./internal/cli/internal_slices_alpha_domain_allowed_test", "internal/slices/alpha/domain", " from its tests", row},
		// A row that is not three fields, code or tests, refuses on its own.
		{"./cmd/slices", "", "", "internal/git both internal/slices/alpha"},
	} {
		t.Run("outside "+c.pkg+" "+c.exceptions, func(t *testing.T) {
			output, err := runCheck(check, dir, append(env, "ARCHITECTURE_OUTSIDE_EXCEPTIONS="+c.exceptions), "--outside", c.pkg)
			if strings.Contains(c.exceptions, "both") {
				expect(t, output, err, true, "a row of outside_exceptions is not")
				return
			}
			importer := module + "/" + strings.TrimPrefix(c.pkg, "./")
			expect(t, output, err, c.imported != "", importer+" imports "+module+"/"+c.imported+c.from+";", "ADR-0053")
		})
	}
}
