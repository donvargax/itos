package proof

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadSummaryOptsInByATopLevelObjectMember(t *testing.T) {
	for _, stdout := range []string{
		"", "   \n", "proof completed", `{"itos_proof":`, `{"itos_proof":{"version":1}} {}`, `{"itos_proof":{"version":1,"judged":01,"unit":"f"}}`,
		"null", "true", "42", `"itos_proof"`, `[{"itos_proof":{"version":1}}]`,
		`{"files":[{"judged":0}],"judged":0}`, `{"provider":{"itos_proof":{"version":1,"judged":0}}}`,
		`{"files":[],"files":[{"judged":0}]}`, `{"ITOS_PROOF":{"version":1,"judged":0}}`,
	} {
		if s, err := ReadSummary([]byte(stdout)); s != nil || err != nil {
			t.Errorf("%q opted in: %+v, %v", stdout, s, err)
		}
	}
}

func TestReadSummaryNormalizes(t *testing.T) {
	for _, c := range []struct{ stdout, want, line string }{
		{` {"itos_proof":{"version":1}} `,
			`{"version":1,"judged":null,"unit":null,"files":null,"killed":null,"survived":null,"excepted":null,"uncovered":null}`,
			"Code proof summary unavailable: provider did not report a summary."},
		{`{"itos_proof":{"version":1,"judged":null,"unit":null,"killed":0}}`,
			`{"version":1,"judged":null,"unit":null,"files":null,"killed":0,"survived":null,"excepted":null,"uncovered":null}`,
			"Code proof summary: killed=0"},
		{`{"itos_proof":{"version":1,"unit":"Branch-case_2"}}`,
			`{"version":1,"judged":null,"unit":"Branch-case_2","files":null,"killed":null,"survived":null,"excepted":null,"uncovered":null}`,
			"Code proof summary: unit=Branch-case_2"},
		{`{"native":1,"native":2,"itos_proof":{"version":1,"Judged":0,"note":1,"note":{"judged":0},"judged":2,"unit":"function","files":9007199254740991,"killed":6,"survived":0,"excepted":22,"uncovered":4}}`,
			`{"version":1,"judged":2,"unit":"function","files":9007199254740991,"killed":6,"survived":0,"excepted":22,"uncovered":4}`,
			"Code proof summary: judged=2; unit=function; files=9007199254740991; killed=6; survived=0; excepted=22; uncovered=4"},
	} {
		s, err := ReadSummary([]byte(c.stdout))
		if err != nil || s == nil {
			t.Errorf("%q: %+v, %v", c.stdout, s, err)
			continue
		}
		got, _ := json.Marshal(s)
		if string(got) != c.want || s.Line() != c.line {
			t.Errorf("%q: %s, %q", c.stdout, got, s.Line())
		}
	}
	if line := (*Summary)(nil).Line(); line != "Code proof summary unavailable: provider did not report a summary." {
		t.Errorf("no summary: %q", line)
	}
}

func TestReadSummaryNamesTheBrokenField(t *testing.T) {
	long := strings.Repeat("a", 65)
	for _, c := range []struct{ stdout, field string }{
		{`{"itos_proof":null}`, "itos_proof"},
		{`{"itos_proof":[]}`, "itos_proof"},
		{`{"itos_proof":"summary"}`, "itos_proof"},
		{`{"itos_proof":1}`, "itos_proof"},
		{`{"itos_proof":{"version":1},"itos_proof":{"version":1}}`, "itos_proof"},
		{`{"itos_proof":{}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":null}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":2}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":"1"}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":1.0}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":1e0}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":1,"version":1}}`, "itos_proof.version"},
		{`{"itos_proof":{"version":1,"judged":1.5,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":0e0,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":-0,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":-1,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":"1","unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":false,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":[],"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":9007199254740992,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":99999999999999999999,"unit":"f"}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"judged":null,"judged":null}}`, "itos_proof.judged"},
		{`{"itos_proof":{"version":1,"files":[{"killed":6}]}}`, "itos_proof.files"},
		{`{"itos_proof":{"version":1,"killed":0.5}}`, "itos_proof.killed"},
		{`{"itos_proof":{"version":1,"survived":1e0}}`, "itos_proof.survived"},
		{`{"itos_proof":{"version":1,"excepted":9007199254740992}}`, "itos_proof.excepted"},
		{`{"itos_proof":{"version":1,"uncovered":false}}`, "itos_proof.uncovered"},
		{`{"itos_proof":{"version":1,"judged":0}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"judged":1,"unit":null}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":""}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"1f"}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"_f"}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"fünction"}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"f g"}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"` + long + `"}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":1}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"judged":null,"unit":false}}`, "itos_proof.unit"},
		{`{"itos_proof":{"version":1,"unit":"f","unit":"f"}}`, "itos_proof.unit"},
	} {
		s, err := ReadSummary([]byte(c.stdout))
		if s != nil || err == nil || err.Field != c.field {
			t.Errorf("%q: %+v, %+v; want the field %s", c.stdout, s, err, c.field)
			continue
		}
		if !strings.HasPrefix(err.Message(), c.field+" ") {
			t.Errorf("%q: the message %q does not begin with its field", c.stdout, err.Message())
		}
	}
	unit := strings.Repeat("a", 64)
	if s, err := ReadSummary([]byte(`{"itos_proof":{"version":1,"unit":"` + unit + `"}}`)); err != nil || s == nil || *s.Unit != unit {
		t.Errorf("a unit of 64 characters: %+v, %v", s, err)
	}
}

func TestReadJudgesAnExitZeroByItsEnvelope(t *testing.T) {
	for _, c := range []struct {
		stdout  string
		outcome Outcome
		noWork  bool
		invalid string
	}{
		{"", Passed, false, ""},
		{`{"itos_proof":{"version":1,"judged":1,"unit":"function","killed":0}}`, Passed, false, ""},
		{`{"itos_proof":{"version":1,"judged":0,"unit":"function","killed":0}}`, Refused, true, ""},
		{`{"itos_proof":{"version":1,"judged":0}}`, CannotRun, false, "itos_proof.unit"},
	} {
		v := Read(0, []byte(c.stdout))
		invalid := ""
		if v.Invalid != nil {
			invalid = v.Invalid.Field
		}
		if v.Outcome != c.outcome || v.NoWork() != c.noWork || invalid != c.invalid || len(v.Problems) != 0 {
			t.Errorf("%q: %+v", c.stdout, v)
		}
		if c.invalid != "" && (v.Summary != nil || !strings.HasPrefix(v.Why, c.invalid+" ")) {
			t.Errorf("%q: %+v", c.stdout, v)
		}
	}
	for _, code := range []int{1, 2, 3} {
		v := Read(code, []byte(`{"schema":1,"ok":false,"itos_proof":{"version":1,"judged":0,"unit":"f"}}`))
		if v.Summary != nil || v.Invalid != nil || v.NoWork() {
			t.Errorf("exit %d read the envelope: %+v", code, v)
		}
	}
}

func TestOutcomeString(t *testing.T) {
	for o, want := range map[Outcome]string{Passed: "passed", Refused: "refused", CannotRun: "cannot-run"} {
		if o.String() != want {
			t.Errorf("%d is %q, not %q", o, o.String(), want)
		}
	}
}
