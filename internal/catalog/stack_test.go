package catalog_test

import (
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

const postgresYAML = `name: postgres
description: PostgreSQL 16 with pgAdmin
category: database
primary: postgres
cli: psql -U ${POSTGRES_USER} ${POSTGRES_DB}
gui:
  service: pgadmin
  url: http://localhost:${PGADMIN_PORT}
  login: ${PGADMIN_EMAIL} / ${PGADMIN_PASSWORD}
url: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}
ports:
  - name: POSTGRES_PORT
    default: 5432
  - name: PGADMIN_PORT
    default: 8080
profiles: [gui]
`

func TestParseStack(t *testing.T) {
	s, err := catalog.ParseStack("postgres", []byte(postgresYAML))
	if err != nil {
		t.Fatalf("ParseStack: %v", err)
	}

	if s.Name != "postgres" || s.Category != catalog.CategoryDatabase || s.Primary != "postgres" {
		t.Errorf("parsed %+v", s)
	}
	if !s.HasGUI() || s.GUI.Service != "pgadmin" {
		t.Errorf("gui = %+v", s.GUI)
	}
	if s.HasGPU() {
		t.Error("postgres should not claim a GPU variant")
	}
	if len(s.Ports) != 2 || s.Ports[0].Name != "POSTGRES_PORT" || s.Ports[0].Default != 5432 {
		t.Errorf("ports = %+v", s.Ports)
	}
	if got := s.PortNames(); strings.Join(got, ",") != "POSTGRES_PORT,PGADMIN_PORT" {
		t.Errorf("PortNames = %v", got)
	}
}

// stacks/pytorch is the stack that shaped the schema: both of its services sit
// behind profiles and share ports, so nothing starts without a profile.
func TestParseStackProfilesAndGPU(t *testing.T) {
	const y = `name: pytorch
description: PyTorch with JupyterLab, CPU or NVIDIA GPU
category: ml
primary: jupyter
cli: /bin/bash
url: http://localhost:${JUPYTER_PORT}
ports:
  - name: JUPYTER_PORT
    default: 8888
profiles: [cpu, gpu]
default_profiles: [cpu]
gpu:
  profile: gpu
  service: jupyter-gpu
`
	s, err := catalog.ParseStack("pytorch", []byte(y))
	if err != nil {
		t.Fatalf("ParseStack: %v", err)
	}

	if len(s.DefaultProfiles) != 1 || s.DefaultProfiles[0] != "cpu" {
		t.Errorf("default_profiles = %v, want [cpu]", s.DefaultProfiles)
	}
	if !s.HasGPU() || s.GPU.Profile != "gpu" || s.GPU.Service != "jupyter-gpu" {
		t.Errorf("gpu = %+v", s.GPU)
	}
	if !s.HasProfile("cpu") || !s.HasProfile("gpu") || s.HasProfile("gui") {
		t.Errorf("profiles = %v", s.Profiles)
	}
}

func TestParseStackRejects(t *testing.T) {
	// Each case is one field mutated out of an otherwise valid stack, and the
	// substring the error must name so the user can find it.
	for name, tc := range map[string]struct{ yaml, want string }{
		"unknown key": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\nprofils: [gui]\n", "profils",
		},
		"name mismatch": {
			"name: other\ndescription: d\ncategory: web\nprimary: p\n", "does not match the directory",
		},
		"bad name": {
			"name: Not_Kebab\ndescription: d\ncategory: web\nprimary: p\n", "kebab-case",
		},
		"missing description": {
			"name: x\ncategory: web\nprimary: p\n", "description is required",
		},
		"unknown category": {
			"name: x\ndescription: d\ncategory: databse\nprimary: p\n", "category",
		},
		"missing primary": {
			"name: x\ndescription: d\ncategory: web\n", "primary is required",
		},
		"port is not an env var": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: http-port\n    default: 80\n",
			"is not an env var name",
		},
		"port out of range": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 99999\n",
			"is not a port",
		},
		"duplicate port": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\n  - name: X_PORT\n    default: 81\n",
			"listed twice",
		},
		"separate gui container that is not behind the gui profile": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\ngui:\n  service: other\n  url: http://localhost\n",
			"not in profiles",
		},
		"gui without a service": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\nprofiles: [gui]\ngui:\n  url: http://localhost\n",
			"gui.service is required",
		},
		"default profile that does not exist": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\nprofiles: [cpu]\ndefault_profiles: [gpu]\n",
			"default_profiles",
		},
		"gpu profile that does not exist": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\nports:\n  - name: X_PORT\n    default: 80\nprofiles: [cpu]\ngpu:\n  profile: gpu\n  service: s\n",
			"gpu.profile",
		},
		"missing url": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nports:\n  - name: X_PORT\n    default: 80\n",
			"url is required",
		},
		"no ports": {
			"name: x\ndescription: d\ncategory: web\nprimary: p\nurl: http://localhost\n",
			"ports is required",
		},
		"not yaml": {"name: [unclosed\n", "yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := catalog.ParseStack("x", []byte(tc.yaml))
			if err == nil {
				t.Fatalf("want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// Every problem should be reported at once — fixing a new stack one error per
// run is a miserable loop.
func TestParseStackReportsEveryProblem(t *testing.T) {
	_, err := catalog.ParseStack("x", []byte("name: x\ncategory: nope\n"))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"description is required", "category", "primary is required", "url is required", "ports is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q:\n%s", want, err)
		}
	}
}

// nginx-static, nginx-proxy-manager and pytorch all serve their web interface
// from the primary service itself. There is no second container to make
// optional, so requiring a gui profile there would be wrong.
func TestGUIOnThePrimaryServiceNeedsNoProfile(t *testing.T) {
	const y = `name: nginx-static
description: nginx serving a static site
category: web
primary: nginx
cli: /bin/sh
gui:
  service: nginx
  url: http://localhost:${NGINX_PORT}
  login: none
url: http://localhost:${NGINX_PORT}
ports:
  - name: NGINX_PORT
    default: 8090
profiles: []
`
	s, err := catalog.ParseStack("nginx-static", []byte(y))
	if err != nil {
		t.Fatalf("ParseStack: %v", err)
	}
	if !s.HasGUI() {
		t.Error("HasGUI = false")
	}
	if len(s.Profiles) != 0 {
		t.Errorf("profiles = %v, want none", s.Profiles)
	}
}

func TestExpand(t *testing.T) {
	env := map[string]string{
		"POSTGRES_USER":     "spinup",
		"POSTGRES_PASSWORD": "s3cret",
		"POSTGRES_PORT":     "5433",
		"POSTGRES_DB":       "app",
	}

	const in = "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}"
	want := "postgres://spinup:s3cret@localhost:5433/app"
	if got := catalog.Expand(in, env); got != want {
		t.Errorf("Expand = %q, want %q", got, want)
	}

	// An unset variable expands to nothing, as it would in Compose.
	if got := catalog.Expand("a${MISSING}b", env); got != "ab" {
		t.Errorf("Expand of an unset var = %q, want %q", got, "ab")
	}
	if got := catalog.Expand("no placeholders", nil); got != "no placeholders" {
		t.Errorf("Expand = %q", got)
	}
}
