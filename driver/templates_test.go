package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmbeddedIndexTemplateRenders(t *testing.T) {
	renderer, err := newTemplateRenderer()
	if err != nil {
		t.Fatalf("newTemplateRenderer() error = %v", err)
	}
	var output bytes.Buffer
	err = renderer.render(&output, "index.html", indexPageData{LinearLimit: 1.5, AngularLimit: 2, RobotID: "robot-1"})
	if err != nil {
		t.Fatalf("render() error = %v", err)
	}
	if !strings.Contains(output.String(), "robot-1") {
		t.Fatalf("rendered template does not include the robot ID: %q", output.String())
	}
}
