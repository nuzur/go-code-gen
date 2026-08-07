package githubactions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// generateWorkflows renders both workflows and returns them keyed by filename.
func generateWorkflows(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	if err := GenerateGitHubActions(context.Background(), scopeParams(root)); err != nil {
		t.Fatalf("GenerateGitHubActions: %v", err)
	}
	dir := filepath.Join(root, "myapp", ".github", "workflows")
	out := map[string]string{}
	for _, name := range []string{"publish-myapp-image.yaml", "publish-myapp-helm.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		out[name] = string(b)
	}
	return out
}

// withoutComments strips whole-line comments, so assertions about what a
// workflow DOES are not satisfied (or tripped) by prose in a comment that
// happens to quote the very command being checked.
func withoutComments(body string) string {
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestWorkflowsAreValidYAML guards against the generator emitting something
// GitHub silently refuses to run. Rendering these with the default delimiters
// used to require escaping every ${{ }}, which is exactly the kind of thing
// that produces subtly malformed YAML.
func TestWorkflowsAreValidYAML(t *testing.T) {
	for name, body := range generateWorkflows(t) {
		t.Run(name, func(t *testing.T) {
			var parsed map[string]any
			if err := yaml.Unmarshal([]byte(body), &parsed); err != nil {
				t.Fatalf("not valid YAML: %v\n---\n%s", err, body)
			}
			// A workflow with no jobs parses fine and does nothing, which is
			// the failure mode a templating slip would actually produce.
			if _, ok := parsed["jobs"]; !ok {
				t.Errorf("workflow has no jobs\n%s", body)
			}
		})
	}
}

// TestImageWorkflowPublishesAnImmutableReference pins the two things that make
// a build pinnable. With only `type=raw,value=latest` there is no immutable
// handle on any build: nothing can be rolled back, and `helm upgrade` with an
// unchanged image string does not even restart the pods.
func TestImageWorkflowPublishesAnImmutableReference(t *testing.T) {
	body := withoutComments(generateWorkflows(t)["publish-myapp-image.yaml"])

	if !strings.Contains(body, "type=sha,format=long") {
		t.Errorf("image workflow publishes no commit-addressed tag\n%s", body)
	}
	// docker/build-push-action only exposes outputs.digest when the step has an
	// id, so without this the digest cannot be consumed by anything.
	if !strings.Contains(body, "id: build") {
		t.Errorf("build step has no id, so steps.build.outputs.digest is unavailable\n%s", body)
	}
	if !strings.Contains(body, "steps.build.outputs.digest") {
		t.Errorf("image workflow never reports the digest\n%s", body)
	}
}

// TestHelmWorkflowUpdatesDependencies covers the step sfapi had to add by hand
// (commit ed65285): helm package fails outright on a chart with a dependencies:
// block whose subcharts are not vendored.
func TestHelmWorkflowUpdatesDependencies(t *testing.T) {
	body := withoutComments(generateWorkflows(t)["publish-myapp-helm.yaml"])
	deps := strings.Index(body, "helm dependency update ")
	pkg := strings.Index(body, "helm package ")
	if deps == -1 {
		t.Fatalf("helm workflow does not update dependencies before packaging\n%s", body)
	}
	if pkg == -1 {
		t.Fatalf("helm workflow never packages the chart\n%s", body)
	}
	if deps > pkg {
		t.Errorf("dependency update must come before packaging (got %d > %d)", deps, pkg)
	}
}

// TestHelmWorkflowReadsChartVersionRobustly guards the version extraction.
//
// The old `grep 'version:' Chart.yaml | tail -n1` works only by accident: grep
// is case-sensitive so apiVersion/appVersion do not match, and dependencies
// happen to be listed ABOVE the chart version. Move the dependencies block
// below `version:` and tail -n1 silently yields a SUBCHART's version, so the
// push uploads a file that does not exist — or worse, the wrong one.
func TestHelmWorkflowReadsChartVersionRobustly(t *testing.T) {
	body := withoutComments(generateWorkflows(t)["publish-myapp-helm.yaml"])
	if strings.Contains(body, "tail -n1") {
		t.Errorf("helm workflow still derives the chart version positionally\n%s", body)
	}
	if !strings.Contains(body, "helm show chart") {
		t.Errorf("helm workflow should read the version from chart metadata\n%s", body)
	}
}
