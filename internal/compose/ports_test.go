package compose

import "testing"

// The shape `docker compose config --format json` actually produces, trimmed
// to the fields HostPorts reads.
const configJSON = `{
  "services": {
    "pgadmin": {"ports": [{"target": 80, "published": "8080", "protocol": "tcp"}]},
    "postgres": {"ports": [{"target": 5432, "published": "5432", "protocol": "tcp"}]}
  }
}`

func TestParseHostPortsIsSortedAndComplete(t *testing.T) {
	got, err := parseHostPorts([]byte(configJSON))
	if err != nil {
		t.Fatalf("parseHostPorts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %+v, want two ports", got)
	}
	// Services come out of a map, so the order has to be imposed, not observed.
	if got[0].Port != 5432 || got[0].Service != "postgres" {
		t.Errorf("first port is %+v, want 5432 from postgres", got[0])
	}
	if got[1].Port != 8080 || got[1].Service != "pgadmin" {
		t.Errorf("second port is %+v, want 8080 from pgadmin", got[1])
	}
}

// A range or an empty published value is compose choosing the port itself.
// There is nothing to check, and reading "8000-8010" as a port number would
// refuse a start over a port that was never requested.
func TestParseHostPortsSkipsWhatComposePicks(t *testing.T) {
	in := `{"services": {"app": {"ports": [
	  {"target": 80, "published": "8000-8010"},
	  {"target": 81, "published": ""},
	  {"target": 82},
	  {"target": 83, "published": "9000"}
	]}}}`

	got, err := parseHostPorts([]byte(in))
	if err != nil {
		t.Fatalf("parseHostPorts: %v", err)
	}
	if len(got) != 1 || got[0].Port != 9000 {
		t.Errorf("got %+v, want only the fixed port 9000", got)
	}
}

// Two services publishing the same host port cannot both be right, but the
// check should ask about it once.
func TestParseHostPortsDeduplicates(t *testing.T) {
	in := `{"services": {
	  "a": {"ports": [{"published": "8080"}]},
	  "b": {"ports": [{"published": "8080"}]}
	}}`

	got, err := parseHostPorts([]byte(in))
	if err != nil {
		t.Fatalf("parseHostPorts: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %+v, want one entry for 8080", got)
	}
}

func TestParseHostPortsRejectsGarbage(t *testing.T) {
	if _, err := parseHostPorts([]byte("not json")); err == nil {
		t.Error("parseHostPorts accepted something that is not JSON")
	}
}

// A stack with no published ports is not an error — several stacks publish
// nothing until a profile is selected.
func TestParseHostPortsAllowsNone(t *testing.T) {
	got, err := parseHostPorts([]byte(`{"services": {"app": {}}}`))
	if err != nil {
		t.Fatalf("parseHostPorts: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}
