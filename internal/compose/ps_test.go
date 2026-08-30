package compose_test

import (
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

// Compose has shipped both shapes of `ps --format json` under v2: a JSON array
// in newer releases, one object per line in older ones. spinup has to read a
// user's compose, not the one it was developed against.
func TestParsePSBothFormats(t *testing.T) {
	const object = `{"Name":"spinup-postgres-postgres-1","Service":"postgres","State":"running","Health":"healthy","Publishers":[{"URL":"0.0.0.0","TargetPort":5432,"PublishedPort":5432,"Protocol":"tcp"}]}`

	for name, out := range map[string]string{
		"ndjson": object + "\n" + `{"Name":"spinup-postgres-pgadmin-1","Service":"pgadmin","State":"running","Health":""}`,
		"array":  "[" + object + `,{"Name":"spinup-postgres-pgadmin-1","Service":"pgadmin","State":"running","Health":""}]`,
	} {
		t.Run(name, func(t *testing.T) {
			containers, err := compose.ParsePS([]byte(out))
			if err != nil {
				t.Fatalf("ParsePS: %v", err)
			}
			if len(containers) != 2 {
				t.Fatalf("parsed %d containers, want 2", len(containers))
			}

			db := containers[0]
			if db.Service != "postgres" || !db.Healthy() {
				t.Errorf("postgres = %+v", db)
			}
			if len(db.Publishers) != 1 || db.Publishers[0].PublishedPort != 5432 {
				t.Errorf("publishers = %+v", db.Publishers)
			}

			// pgAdmin has no healthcheck: running is all the health there is.
			if gui := containers[1]; !gui.Healthy() {
				t.Errorf("a running container with no healthcheck should count as healthy: %+v", gui)
			}
		})
	}
}

func TestParsePSEmpty(t *testing.T) {
	containers, err := compose.ParsePS([]byte("  \n"))
	if err != nil || len(containers) != 0 {
		t.Errorf("ParsePS of empty output = %v, %v", containers, err)
	}
}

func TestContainerHealth(t *testing.T) {
	for name, tc := range map[string]struct {
		c    compose.Container
		want bool
	}{
		"healthy":            {compose.Container{State: "running", Health: "healthy"}, true},
		"running, no check":  {compose.Container{State: "running"}, true},
		"still starting":     {compose.Container{State: "running", Health: "starting"}, false},
		"unhealthy":          {compose.Container{State: "running", Health: "unhealthy"}, false},
		"exited":             {compose.Container{State: "exited"}, false},
		"healthy but not up": {compose.Container{State: "exited", Health: "healthy"}, false},
	} {
		if got := tc.c.Healthy(); got != tc.want {
			t.Errorf("%s: Healthy = %v, want %v", name, got, tc.want)
		}
	}
}
